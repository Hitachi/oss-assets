package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authpb "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

// ============================================================
// Modes
// ============================================================

type Mode int

const (
    AccessTokenExchanger Mode = 1 + iota
    AccessTokenValidatorWithDecision
)

func (m Mode) String() string {
    switch m {
    case AccessTokenExchanger:
        return "access_token_exchanger"
    case AccessTokenValidatorWithDecision:
        return "access_token_validator_with_decision"
    default:
        return fmt.Sprintf("UNKNOWN(%d)", m)
    }
}

func parseMode(s string) (Mode, error) {
    switch strings.ToLower(s) {
    case "access_token_exchanger":
        return AccessTokenExchanger, nil
    case "access_token_validator_with_decision":
        return AccessTokenValidatorWithDecision, nil
    default:
        return 0, fmt.Errorf("unknown mode %q. must be one of: access_token_exchanger, access_token_validator_with_decision", s)
    }
}

// ============================================================
// Config
// ============================================================

type Config struct {
    jwtSource                *workloadapi.JWTSource
    downstreamAudience       string
    svidAudienceForKeycloak  string
    keycloakTokenEndpoint    string
    mode                     Mode
    httpClient               *http.Client
}

// AuthServer implements Envoy external authorization (ext_authz).
type AuthServer struct {
    mu     sync.RWMutex // protects jwtSource updates
    config *Config
}

// NewAuthServer initializes the authorization server instance.
func NewAuthServer(
    downstreamAudience string,
    svidAudienceForKeycloak string,
    modeStr string,
    keycloakTokenEndpoint string,
    jwtSource *workloadapi.JWTSource,
) (*AuthServer, error) {
    cfg := &Config{
        jwtSource:               jwtSource, // may be nil; can be injected later
        downstreamAudience:      downstreamAudience,
        svidAudienceForKeycloak: svidAudienceForKeycloak,
        keycloakTokenEndpoint:   keycloakTokenEndpoint,
        mode:                    AccessTokenExchanger,
        httpClient:              &http.Client{Timeout: 30 * time.Second},
    }
    if modeStr != "" {
        m, err := parseMode(modeStr)
        if err != nil {
            return nil, err
        }
        cfg.mode = m
    }
    log.Printf("[INFO] AuthServer initialized (mode=%s)", cfg.mode)
    return &AuthServer{config: cfg}, nil
}

// Mode returns the current operating mode.
func (a *AuthServer) Mode() Mode {
    return a.config.mode
}

// NeedsJWTSource returns true only when RFC 8693 Token Exchange is enabled.
func (a *AuthServer) NeedsJWTSource() bool {
    return a.config.mode == AccessTokenExchanger
}

// SetJWTSource allows late injection of JWTSource after server start.
func (a *AuthServer) SetJWTSource(js *workloadapi.JWTSource) {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.config.jwtSource = js
    log.Printf("[INFO] JWTSource set on AuthServer")
}

// ============================================================
// MCP resource_metadata URL builder (STRICT: no fallback)
// ============================================================

// buildResourceMetadataURLStrict builds <scheme>://<host>/.well-known/oauth-protected-resource
// using ONLY request headers. No fallback/defaults are used.
//   - scheme: X-Forwarded-Proto (required)
//   - host: :authority (H2) or Host (H1) (required)
//
// Returns empty string if scheme or host is missing.
func buildResourceMetadataURLStrict(req *authpb.CheckRequest) string {
    httpReq := req.GetAttributes().GetRequest().GetHttp()

    // Normalize headers to lower-case
    headers := map[string]string{}
    if httpReq.GetHeaders() != nil {
        for k, v := range httpReq.GetHeaders() {
            headers[strings.ToLower(k)] = v
        }
    }

    scheme := headers["x-forwarded-proto"]
    host := headers[":authority"]
    if host == "" {
        host = headers["host"]
    }
    if scheme == "" ||
        host == "" {
        return ""
    }
    return scheme + "://" + host + "/.well-known/oauth-protected-resource"
}

// ============================================================
// Check entrypoint
// ============================================================

func (a *AuthServer) Check(ctx context.Context, req *authpb.CheckRequest) (*authpb.CheckResponse, error) {
    // Basic request info (path/method) for observability
    path := req.Attributes.GetRequest().GetHttp().GetPath()
    method := strings.ToLower(req.Attributes.GetRequest().GetHttp().GetMethod())

    // Verify Authorization header presence and extract bearer token
    authHeader := req.Attributes.Request.Http.Headers["authorization"]
    token, ok := parseBearerToken(authHeader)
    if !ok {
        // Build an MCP-compliant challenge strictly from headers, if possible
        mcpMeta := buildResourceMetadataURLStrict(req)
        if mcpMeta == "" {
            log.Printf("[WARN] missing/malformed Authorization AND insufficient headers to build resource_metadata (need X-Forwarded-Proto + :authority/host)")
            // Return 401 without challenge when we cannot construct resource_metadata
            return a.unauthorizedResponseWithoutChallenge("invalid or missing authorization header"), nil
        }
        log.Printf("[WARN] missing/malformed Authorization; resource_metadata=%s", mcpMeta)
        return a.unauthorizedResponseWithMCP(mcpMeta, "invalid or missing authorization header"), nil
    }

    switch a.config.mode {
    case AccessTokenExchanger:
        // RFC 8693 Token Exchange
        exchanged, err := a.exchangeAccessToken(ctx, token)
        if err != nil {
            if errors.Is(err, ErrJWTSourceNotReady) {
                // Missing JWT-SVID source is a server-side transient failure -> return 503 without challenge
                log.Printf("[WARN] token exchange aborted: jwt-source-not-ready -> 503")
                return a.serviceUnavailableResponse("jwt-source-not-ready", /*retryAfterSeconds*/ 30), nil
            }
            log.Printf("[ERROR] token exchange failed: %v", err)
            // Other exchange failures -> return 403
            return a.forbiddenResponse(err.Error()), nil
        }
        // Rewrite Authorization header with the downstream token
        headers := []*core.HeaderValueOption{{
            Append: &wrappers.BoolValue{Value: false},
            Header: &core.HeaderValue{
                Key:   "authorization",
                Value: fmt.Sprintf("Bearer %s", exchanged),
            },
        }}
        log.Printf("[DEBUG] token exchange succeeded; header rewritten")
        return a.okResponse(headers), nil

    case AccessTokenValidatorWithDecision:
        // (1) Perform a local audience-only check (no signature verification)
        audOK, err := checkAudienceOnly(token, a.config.svidAudienceForKeycloak)
        if err != nil || !audOK {
            log.Printf("[WARN] audience check failed: audOK=%v err=%v", audOK, err)
            // Audience mismatch is unlikely to be fixed by re-authentication -> 403
            return a.forbiddenResponse("audience check failed"), nil
        }
        // (2) Delegate authorization decision to Keycloak UMA
        decision, derr := a.delegateDecision(ctx, token, path, method)
        if derr != nil {
            log.Printf("[ERROR] UMA decision delegation failed: %v", derr)
            return a.forbiddenResponse(derr.Error()), nil
        }
        if decision {
            log.Printf("[DEBUG] decision=ALLOW")
            return a.okResponse(nil), nil
        }
        log.Printf("[DEBUG] decision=DENY")
        return a.forbiddenResponse("PERMISSION_DENIED"), nil

    default:
        err := fmt.Errorf("unknown server mode: %s", a.config.mode)
        log.Printf("[ERROR] %v", err)
        return nil, err
    }
}

// ============================================================
// RFC8693 Token Exchange (Keycloak) - SPIFFE Federated client assertion
// ============================================================

var ErrJWTSourceNotReady = errors.New("jwt-source-not-ready")

func (a *AuthServer) exchangeAccessToken(ctx context.Context, subjectToken string) (string, error) {
    // Ensure JWTSource is provided; otherwise return 503-equivalent error
    a.mu.RLock()
    js := a.config.jwtSource
    a.mu.RUnlock()
    if js == nil {
        return "", ErrJWTSourceNotReady
    }

    realmBase, err := extractRealmBase(a.config.keycloakTokenEndpoint)
    if err != nil {
        log.Printf("[ERROR] extractRealmBase failed: %v", err)
        return "", err
    }

    // Obtain a JWT-SVID to authenticate as a federated client to Keycloak
    svid, err := js.FetchJWTSVID(ctx, jwtsvid.Params{
        Audience: realmBase,
    })
    if err != nil {
        return "", fmt.Errorf("fetch jwt-svid failed: %w", err)
    }
    svidJWT := svid.Marshal()

    // --- RFC 8693 form body ---
    form := url.Values{}
    form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
    form.Set("subject_token", subjectToken)
    form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
    form.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
    // downstreamAudience -> audience for the downstream service
    form.Set("scope", a.config.downstreamAudience)

    // --- Federated JWT client authentication (SPIFFE draft) ---
    // client_id uses the last SPIFFE segment (e.g., "frontend"), not the full SPIFFE ID
    form.Set("client_id", a.config.svidAudienceForKeycloak)
    form.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-spiffe")
    form.Set("client_assertion", svidJWT) // send the JWT-SVID as client assertion as-is

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.keycloakTokenEndpoint, strings.NewReader(form.Encode()))
    if err != nil {
        return "", err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    // Request log (sensitive values masked)
    logHTTPRequest("token-exchange", req.Method, req.URL.String(), req.Header, form)

    // Send & measure
    start := time.Now()
    resp, err := a.config.httpClient.Do(req)
    elapsed := time.Since(start)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    // Response log (sensitive values masked)
    resBodyRaw, _ := io.ReadAll(resp.Body)
    redacted := redactJSONBody(resBodyRaw)
    logHTTPResponse("token-exchange", resp.StatusCode, resp.Header, redacted, elapsed)

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("token-exchange response %d: %s", resp.StatusCode, string(resBodyRaw))
    }

    var body struct {
        AccessToken string `json:"access_token"`
        TokenType   string `json:"token_type"`
        ExpiresIn   int64  `json:"expires_in"`
    }
    if err := json.Unmarshal(resBodyRaw, &body); err != nil {
        return "", err
    }
    if body.AccessToken == "" {
        return "", fmt.Errorf("empty access_token in token-exchange response")
    }
    return body.AccessToken, nil
}

// ============================================================
// UMA decision delegation (Keycloak)
// ============================================================

func (a *AuthServer) delegateDecision(ctx context.Context, userAccessToken string, resourcePath string, method string) (bool, error) {
    form := url.Values{}
    form.Set("grant_type", "urn:ietf:params:oauth:grant-type:uma-ticket")
    // audience -> Keycloak resource server (client_id)
    form.Set("audience", a.config.svidAudienceForKeycloak)
    form.Set("response_mode", "decision")

    scope := method
    perm := fmt.Sprintf("%s#%s", resourcePath, scope)
    form.Add("permission", perm)

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.keycloakTokenEndpoint, strings.NewReader(form.Encode()))
    if err != nil {
        return false, err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", userAccessToken)) // masked in logs

    // Request log (sensitive values masked)
    logHTTPRequest("uma-decision", req.Method, req.URL.String(), req.Header, form)

    start := time.Now()
    resp, err := a.config.httpClient.Do(req)
    elapsed := time.Since(start)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    // Response log (sensitive values masked)
    resBodyRaw, _ := io.ReadAll(resp.Body)
    redacted := redactJSONBody(resBodyRaw)
    logHTTPResponse("uma-decision", resp.StatusCode, resp.Header, redacted, elapsed)

    if resp.StatusCode != http.StatusOK {
        return false, fmt.Errorf("decision response %d: %s", resp.StatusCode, string(resBodyRaw))
    }
    var body struct {
        Result bool `json:"result"`
    }
    if err := json.Unmarshal(resBodyRaw, &body); err != nil {
        return false, err
    }
    return body.Result, nil
}

// ============================================================
// Envoy responses
// ============================================================

func (a *AuthServer) okResponse(headers []*core.HeaderValueOption) *authpb.CheckResponse {
    return &authpb.CheckResponse{
        Status: &statuspb.Status{Code: 0}, // OK
        HttpResponse: &authpb.CheckResponse_OkResponse{
            OkResponse: &authpb.OkHttpResponse{
                Headers: headers,
            },
        },
    }
}

func (a *AuthServer) forbiddenResponse(body string) *authpb.CheckResponse {
    return &authpb.CheckResponse{
        Status: &statuspb.Status{Code: 7}, // PERMISSION_DENIED
        HttpResponse: &authpb.CheckResponse_DeniedResponse{
            DeniedResponse: &authpb.DeniedHttpResponse{
                Status: &envoy_type.HttpStatus{
                    Code: envoy_type.StatusCode_Forbidden,
                },
                Body: body,
            },
        },
    }
}

// Return 401 Unauthorized with WWW-Authenticate challenge (MCP-compliant)
func (a *AuthServer) unauthorizedResponseWithMCP(resourceMetadataURL string, body string) *authpb.CheckResponse {
    challenge := `Bearer resource_metadata="` + resourceMetadataURL + `"`
    headers := []*core.HeaderValueOption{
        {
            Append: &wrappers.BoolValue{Value: false},
            Header: &core.HeaderValue{
                Key:   "www-authenticate",
                Value: challenge,
            },
        },
    }
    return &authpb.CheckResponse{
        Status: &statuspb.Status{Code: 16},
        HttpResponse: &authpb.CheckResponse_DeniedResponse{
            DeniedResponse: &authpb.DeniedHttpResponse{
                Status: &envoy_type.HttpStatus{
                    Code: envoy_type.StatusCode_Unauthorized, // 401
                },
                Headers: headers,
                Body:    body,
            },
        },
    }
}

// Return 401 Unauthorized without challenge
func (a *AuthServer) unauthorizedResponseWithoutChallenge(body string) *authpb.CheckResponse {
    return &authpb.CheckResponse{
        Status: &statuspb.Status{Code: 16},
        HttpResponse: &authpb.CheckResponse_DeniedResponse{
            DeniedResponse: &authpb.DeniedHttpResponse{
                Status: &envoy_type.HttpStatus{
                    Code: envoy_type.StatusCode_Unauthorized,
                },
                Body: body,
            },
        },
    }
}

// Return 503 Service Unavailable (no challenge; optional Retry-After)
func (a *AuthServer) serviceUnavailableResponse(body string, retryAfterSeconds int) *authpb.CheckResponse {
    headers := []*core.HeaderValueOption{}
    if retryAfterSeconds > 0 {
        headers = append(headers, &core.HeaderValueOption{
            Append: &wrappers.BoolValue{Value: false},
            Header: &core.HeaderValue{
                Key:   "retry-after",
                Value: fmt.Sprintf("%d", retryAfterSeconds),
            },
        })
    }
    return &authpb.CheckResponse{
        Status: &statuspb.Status{Code: 14},
        HttpResponse: &authpb.CheckResponse_DeniedResponse{
            DeniedResponse: &authpb.DeniedHttpResponse{
                Status: &envoy_type.HttpStatus{
                    Code: envoy_type.StatusCode_ServiceUnavailable, // 503
                },
                Headers: headers,
                Body:    body,
            },
        },
    }
}

// ============================================================
// Helpers (masking, parsing, audience check, logging)
// ============================================================

func maskToken(s string) string {
    if s == "" {
        return ""
    }
    if strings.HasPrefix(strings.ToLower(s), "bearer ") {
        return "Bearer " + maskToken(strings.TrimSpace(s[7:]))
    }
    if len(s) <= 10 {
        return "****"
    }
    return s[:4] + "..." + s[len(s)-4:]
}

func redactHeaders(h http.Header) map[string][]string {
    out := make(map[string][]string, len(h))
    for k, v := range h {
        lk := strings.ToLower(k)
        switch lk {
        default:
            out[k] = v
        }
    }
    return out
}

func redactForm(form url.Values) url.Values {
    cp := url.Values{}
    for k, vv := range form {
        lk := strings.ToLower(k)
        switch lk {
        default:
            cp[k] = vv
        }
    }
    return cp
}

func redactJSONBody(b []byte) string {
    var any map[string]interface{}
    if err := json.Unmarshal(b, &any); err != nil {
        return string(b)
    }
    // No specific keys redacted here (add to `keys` as needed)
    keys := []string{}
    for _, k := range keys {
        if v, ok := any[k]; ok {
            if s, ok2 := v.(string); ok2 {
                any[k] = maskToken(s)
            } else {
                any[k] = "<redacted>"
            }
        }
    }
    out, _ := json.Marshal(any)
    return string(out)
}

func logHTTPRequest(tag string, method string, rawURL string, hdr http.Header, form url.Values) {
    log.Printf("[HTTP-REQ][%s] %s %s", tag, method, rawURL)
    log.Printf("[HTTP-REQ][%s] headers=%v", tag, redactHeaders(hdr))
    if form != nil {
        log.Printf("[HTTP-REQ][%s] form=%v", tag, redactForm(form))
    }
}

func logHTTPResponse(tag string, status int, hdr http.Header, bodyRedacted string, elapsed time.Duration) {
    log.Printf("[HTTP-RES][%s] status=%d elapsed=%s", tag, status, elapsed)
    log.Printf("[HTTP-RES][%s] headers=%v", tag, redactHeaders(hdr))
    if len(bodyRedacted) > 4096 {
        bodyRedacted = bodyRedacted[:4096] + "...(truncated)"
    }
    log.Printf("[HTTP-RES][%s] body=%s", tag, bodyRedacted)
}

// Extract Bearer token from Authorization header
func parseBearerToken(h string) (string, bool) {
    h = strings.TrimSpace(h)
    if h == "" {
        return "", false
    }
    if len(h) < 7 ||
        !strings.EqualFold(h[:6], "bearer") {
        return "", false
    }
    rest := strings.TrimSpace(h[6:])
    if rest == "" {
        return "", false
    }
    return rest, true
}

// Audience-only check (no signature verification).
func checkAudienceOnly(jwt string, expectedAud string) (bool, error) {
    parts := strings.Split(jwt, ".")
    if len(parts) < 2 {
        return false, fmt.Errorf("invalid jwt format")
    }
    payloadB64 := parts[1]
    if m := len(payloadB64) % 4; m != 0 {
        payloadB64 += strings.Repeat("=", 4-m)
    }
    payloadBytes, err := base64.URLEncoding.DecodeString(payloadB64)
    if err != nil {
        return false, fmt.Errorf("decode payload failed: %w", err)
    }
    var claims map[string]interface{}
    if err := json.Unmarshal(payloadBytes, &claims); err != nil {
        return false, fmt.Errorf("unmarshal claims failed: %w", err)
    }
    aud, ok := claims["aud"]
    if !ok {
        return false, nil
    }
    switch v := aud.(type) {
    case string:
        return v == expectedAud, nil
    case []interface{}:
        for _, x := range v {
            if xs, ok := x.(string); ok && xs == expectedAud {
                return true, nil
            }
        }
        return false, nil
    default:
        return false, nil
    }
}

// extractRealmBase trims the standard suffix from Keycloak token endpoint and returns the realm base URL.
// Expected format:
//   https://<host>/realms/<realm>/protocol/openid-connect/token
func extractRealmBase(tokenEndpoint string) (string, error) {
    const suffix = "/protocol/openid-connect/token"
    if !strings.HasSuffix(strings.ToLower(tokenEndpoint), suffix) {
        return "", fmt.Errorf("unexpected token endpoint format: %s", tokenEndpoint)
    }
    // Remove the suffix and return realm base
    return tokenEndpoint[:len(tokenEndpoint)-len(suffix)], nil
}