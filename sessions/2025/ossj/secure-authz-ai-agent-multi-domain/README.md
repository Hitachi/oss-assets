# Secure Authorization for AI Agents Operating in Multi-Domain Environments – Demo

This repository contains the demonstration assets for the session **"Secure Authorization for AI Agents Operating in Multi-Domain Environments."**

## Overview

The demo showcases how AI agents can securely collaborate across multiple trust domains using open standards such as OAuth 2.1, MCP (Model Context Protocol), A2A (Agent-to-Agent), and federated authorization flows with Keycloak. It implements cross-domain token exchange and JWT-based identity propagation, demonstrating practical solutions for scalable and interoperable agent infrastructures.

## Directory Structure

```
demo/
├── Dockerfile
├── envoy-egress-a-config.yaml
├── envoy-inbound-a-config.yaml
├── envoy-inbound-b-config.yaml
├── kc-secret-a.yaml
├── kc-secret-b.yaml
├── keycloak.yaml
├── mcp-inspector.yaml
├── mcp-server-a.yaml
├── mcp-server-b.yaml
├── package-lock.json
├── package.json
├── realm-a-realm.json
├── realm-b-realm.json
├── server.js
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

- **Keycloak**: Open-source IAM solution, deployed in two domains (A and B) for federated authorization.
- **MCP Servers (A & B)**: Sample servers implementing the MCP protocol for agent interactions.
- **Envoy**: Serves as an external authorization filter. It forwards incoming requests to either the introspection sidecar for token validation or to the token broker sidecar for token exchange and JWT-based identity propagation, depending on the authorization flow required.
- **Introspection Sidecar**: Python service that validates OAuth tokens and returns authorization decisions to Envoy.
- **Token Broker Sidecar**: Python service that handles OAuth 2.0 Token Exchange (RFC 8693) and JWT-based identity propagation (RFC 7523), enabling secure cross-domain authorization.
- **MCP Inspector**: Client for initiating authorization flows and cross-domain interactions.
- **Realm Configurations**: `realm-a-realm.json` and `realm-b-realm.json` define Keycloak realms for each trust domain.
- **Secrets & Configs**: YAML files for Keycloak secrets and Envoy configuration.

