
# Securing AI Agent Infrastructure: AuthN/AuthZ Patterns for MCP and A2A — Minimal Demo Assets

This folder contains the **essential, reproducible assets** for the KubeCon NA 2025 session demo.
The focus is on **Envoy external authorization (ext_authz)** patterns for:
- **Inbound**: UMA decision delegation to Keycloak.
- **Outbound**: RFC 8693 **Token Exchange** for downstream services.
- **MCP**: Proper `WWW-Authenticate: Bearer resource_metadata=...` challenges and a simple MCP response path.

---

## Directory Layout

```
sessions/2025/kubecon-na/securing-ai-agent-infra-authn-authz-mcp-a2a/
├── README.md                    # this file
└── demo/
    ├── envoy-jwt-auth-helper/   # ext_authz helper (Go)
    │   ├── main.go
    │   └── pkg/auth/ext_auth_server.go
    ├── config/
    │   ├── keycloak.yaml        # demo Keycloak manifests
    │   └── demo-realm.template.json  # example realm
    └── k8s/
        ├── frontend/
        │   ├── frontend-deployment.yaml
        │   └── config/
        │       ├── envoy.yaml
        │       ├── envoy-jwt-auth-helper1.conf   # inbound (UMA decision)
        │       ├── envoy-jwt-auth-helper2.conf   # outbound (token exchange)
        │       └── proxy.conf                    # nginx: /.well-known + /mcp proxy
        └── backend/
            ├── backend-deployment.yaml
            └── config/
                ├── envoy.yaml
                ├── envoy-jwt-auth-helper.conf    # inbound decision helper for backend
                ├── backend-mcp.conf              # nginx: serve mcp.json with application/mcp+json
                └── mcp.json
```

## Prerequisites

- Kubernetes cluster (kind or minikube recommended).
- Docker CLI (or compatible build tool).
- SPIRE (SPIFFE) quickstart applied separately (SDS used by Envoy). Obtain from upstream.
- Keycloak 26.x (container image) for UMA and token operations.

> SPIRE examples: https://github.com/spiffe/spire-examples
> Envoy docs: https://www.envoyproxy.io/
> Keycloak: https://www.keycloak.org/

---

## Configuration

- **Keycloak token endpoint**: set in `envoy-jwt-auth-helper*.conf` files (`keycloak_token_endpoint`).
- **SPIFFE IDs**: examples use `spiffe://example.org/...`. Replace with your own IDs where required.
- **SPIRE Workload socket**: `unix:///run/spire/sockets/agent.sock` (adjust if your agent differs).
- **Downstream audience**:
  - Inbound decision mode: `envoy-jwt-auth-helper1.conf` (frontend) and backend helper conf.
  - Outbound token exchange: `envoy-jwt-auth-helper2.conf` (frontend) expects the backend audience.
