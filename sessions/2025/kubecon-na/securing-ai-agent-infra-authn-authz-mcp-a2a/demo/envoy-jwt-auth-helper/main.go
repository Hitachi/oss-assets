package main

import (
    "context"
    "errors"
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "time"

    authpb "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
    "google.golang.org/grpc"

    myauth "github.com/spiffe/envoy-jwt-auth-helper/pkg/auth"
    "github.com/hashicorp/hcl"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

type Config struct {
    Listen                  string `hcl:"listen"`
    Mode                    string `hcl:"mode"`
    DownstreamAudience      string `hcl:"downstream_audience"`
    SVIDAudienceForKeycloak string `hcl:"svid_audience_for_keycloak"`
    KeycloakTokenEndpoint   string `hcl:"keycloak_token_endpoint"`
    WorkloadSocket          string `hcl:"workload_socket"`
}

// ----------------------------------------
// Read and parse HCL config file into Config
func parseConfigFile(path string) (*Config, error) {
    log.Printf("[INFO] loading config from %s", path)
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }
    var cfg Config
    if err := hcl.Unmarshal(b, &cfg); err != nil {
        return nil, fmt.Errorf("hcl unmarshal: %w", err)
    }
    return &cfg, nil
}

// ----------------------------------------
// Validate required fields and set sensible defaults
func validate(cfg *Config) error {
    if cfg.Listen == "" {
        // If not set, fall back to :9021 (intended for decision mode by default)
        cfg.Listen = ":9021"
        log.Printf("[WARN] cfg.listen is empty -> defaulting to %s", cfg.Listen)
    }
    if cfg.WorkloadSocket == "" {
        cfg.WorkloadSocket = "unix:///run/spire/sockets/agent.sock"
        log.Printf("[WARN] cfg.workload_socket empty -> defaulting to %s", cfg.WorkloadSocket)
    }
    if cfg.Mode == "" {
        cfg.Mode = "access_token_exchanger"
        log.Printf("[WARN] cfg.mode empty -> defaulting to %s", cfg.Mode)
    }
    if cfg.SVIDAudienceForKeycloak == "" {
        return fmt.Errorf("svid_audience_for_keycloak is required")
    }
    if cfg.KeycloakTokenEndpoint == "" {
        return fmt.Errorf("keycloak_token_endpoint is required")
    }
    if cfg.Mode == "access_token_exchanger" && cfg.DownstreamAudience == "" {
        return fmt.Errorf("downstream_audience is required in access_token_exchanger mode")
    }
    return nil
}

// Minimal gRPC logging interceptor (handy to see ext_authz calls)
func unaryLoggingInterceptor(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error) {
    start := time.Now()
    log.Printf("[DEBUG] gRPC call start: method=%s", info.FullMethod)
    resp, err := handler(ctx, req)
    dur := time.Since(start)
    if err != nil {
        log.Printf("[ERROR] gRPC call error: method=%s err=%v dur=%s", info.FullMethod, err, dur)
    } else {
        log.Printf("[DEBUG] gRPC call ok: method=%s dur=%s", info.FullMethod, dur)
    }
    return resp, err
}

func main() {
    // In production you may require -config (fatal if unspecified)
    configPath := flag.String("config", "/run/auth-helper/config/envoy-jwt-auth-helper.conf", "Path to the config file")
    flag.Parse()

    log.Printf("[INFO] starting auth-helper pid=%d", os.Getpid())
    log.Printf("[INFO] using configPath=%s", *configPath)

    // 1) Load config and validate
    cfg, err := parseConfigFile(*configPath)
    if err != nil {
        log.Fatalf("[ERROR] config parse failed: %v", err)
    }
    if err := validate(cfg); err != nil {
        log.Fatalf("[ERROR] config validation failed: %v", err)
    }
    log.Printf("[INFO] config: listen=%q mode=%q workload_socket=%q audience=%q keycloak=%q",
        cfg.Listen, cfg.Mode, cfg.WorkloadSocket, cfg.DownstreamAudience, cfg.KeycloakTokenEndpoint)

    // 2) Start the gRPC server and bind the socket first (ensure the port is open early)
    lis, err := net.Listen("tcp", cfg.Listen)
    if err != nil {
        log.Fatalf("[ERROR] listen(%s) failed: %v", cfg.Listen, err)
    }
    log.Printf("[INFO] tcp socket bound on %s", cfg.Listen)

    grpcSrv := grpc.NewServer(
        grpc.UnaryInterceptor(unaryLoggingInterceptor),
        grpc.MaxConcurrentStreams(10),
    )

    // Design for late JWTSource injection (the myauth side exposes a setter)
    srv, err := myauth.NewAuthServer(
        cfg.DownstreamAudience,
        cfg.SVIDAudienceForKeycloak,
        cfg.Mode,
        cfg.KeycloakTokenEndpoint,
        nil, // initially nil; will be supplied later if needed
    )
    if err != nil {
        log.Fatalf("[ERROR] NewAuthServer error: %v", err)
    }
    authpb.RegisterAuthorizationServer(grpcSrv, srv)

    errCh := make(chan error, 1)
    go func() {
        log.Printf("[INFO] ext_authz gRPC server serving on %s (mode=%s)", cfg.Listen, cfg.Mode)
        errCh <- grpcSrv.Serve(lis)
    }()

    // 3) Initialize JWTSource only when running in token exchange mode
    if srv.NeedsJWTSource() {
        go func() {
            for {
                ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
                log.Printf("[INFO] initializing JWTSource addr=%s ...", cfg.WorkloadSocket)
                js, err := workloadapi.NewJWTSource(ctx,
                    workloadapi.WithClientOptions(workloadapi.WithAddr(cfg.WorkloadSocket)))
                cancel()
                if err != nil {
                    log.Printf("[WARN] JWTSource init failed: %v (retry in 5s)", err)
                    time.Sleep(5 * time.Second)
                    continue
                }
                log.Printf("[INFO] JWTSource ready")
                // Supply JWTSource via myauth.SetJWTSource(*workloadapi.JWTSource)
                srv.SetJWTSource(js)
                return
            }
        }()
    } else {
        log.Printf("[INFO] JWTSource not required in mode=%s", srv.Mode())
    }

    // 4) Signal handling with graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-sigCh:
        log.Printf("[INFO] received signal: %s; shutting down...", sig)
        done := make(chan struct{}, 1)
        go func() {
            grpcSrv.GracefulStop()
            done <- struct{}{}
        }()
        select {
        case <-done:
            log.Printf("[INFO] grpc server stopped gracefully")
        case <-time.After(5 * time.Second):
            log.Printf("[WARN] graceful stop timed out; forcing stop")
            grpcSrv.Stop()
        }
    case e := <-errCh:
        // If Serve returned, log the cause
        if e != nil && !errors.Is(e, grpc.ErrServerStopped) {
            log.Printf("[ERROR] grpc serve error: %v", e)
        }
    }
}