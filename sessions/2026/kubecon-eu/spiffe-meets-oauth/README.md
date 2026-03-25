# SPIFFE Meets OAuth: Federated Identity for Cloud Native Workloads – Demo

This repository contains the demonstration assets for the KubeCon Europe session  
**“SPIFFE Meets OAuth: Federated Identity for Cloud Native Workloads.”**

Session page: https://sched.co/2CW5g

## Overview

This demo shows how cloud native workloads can securely access APIs across multiple trust domains by combining **SPIFFE-based workload identity** with **OAuth-based authorization**.

Each trust domain operates its own identity and authorization infrastructure, while federation is achieved using open standards such as OAuth token exchange, JWT authorization grants, and SPIFFE federation. The demo focuses on **secure authorization across trust boundaries**, without introducing a centralized authorization server.

## Directory Structure

```text
demo/
├── api-server-a.yaml
├── api-server-b.yaml
├── envoy-egress-a-config.yaml
├── envoy-inbound-a-config.yaml
├── envoy-inbound-b-config.yaml
├── keycloak.yaml
├── keycloak-a.yaml
├── keycloak-b.yaml
├── kc-secret-a.yaml
├── kc-secret-b.yaml
├── values-a.yaml
├── values-b.yaml
├── server.js
├── package.json
├── package-lock.json
│
├── introspection-sidecar/
│   ├── app.py
│   ├── Dockerfile
│   └── requirements.txt
│
└── token-broker-sidecar/
    ├── app.py
    ├── Dockerfile
    └── requirements.txt
```

## Components

- **Keycloak**  
  Open-source IAM solution deployed independently in Trust Domain A and Trust Domain B.
  Each instance operates as an OAuth Authorization Server and supports token exchange,
  JWT authorization grant, and SPIFFE-based client authentication.

- **API Server A / API Server B**  
  Simple HTTP services used to demonstrate cross-domain API access.
  API Server A invokes API Server B as a downstream service.

- **Envoy**  
  Acts as the policy enforcement point using the external authorization (`ext_authz`)
  filter.  
  - Inbound Envoy enforces authorization via token introspection.
  - Egress Envoy (in Trust Domain A) invokes the token broker for cross-domain access.

- **Introspection Sidecar**  
  A Python service implementing OAuth 2.0 Token Introspection (RFC 7662).
  It validates access tokens and returns authorization decisions to Envoy.

- **Token Broker Sidecar**  
  A Python service that performs:
  - OAuth 2.0 Token Exchange (RFC 8693) in the source trust domain
  - JWT Authorization Grant (RFC 7523) in the target trust domain  
  This enables secure authorization when accessing APIs in another trust domain.

- **SPIRE / SPIFFE Federation**  
  Each trust domain issues its own SPIFFE identities.
  Federation allows workloads to authenticate across trust domains without
  sharing a single control plane.

## High-Level Flow

1. A request is sent to API Server A.
2. Envoy inbound in Trust Domain A validates the access token via introspection.
3. API Server A calls API Server B through Envoy egress.
4. Envoy egress invokes the token broker sidecar.
5. The token broker exchanges and re-mints the token for Trust Domain B.
6. Envoy inbound in Trust Domain B validates the new token via introspection.
7. API Server B processes the request.

## Disclaimer

This repository is intended for demonstration purposes only.

- It is not a production reference architecture.
- Security hardening and operational concerns are intentionally simplified.
- Configuration values may need to be adapted to your environment.
