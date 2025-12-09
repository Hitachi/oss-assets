#!/bin/bash

set -euo pipefail

# Configuration
NAMESPACE="common-central"
PROXY_NAME="keycloak_proxy"
TOXIC_NAME="demo_latency"
LATENCY_MS=100
JITTER_MS=20

# Retrieve the Toxiproxy Pod name
POD=$(kubectl get pod -n "$NAMESPACE" -l app=toxiproxy -o jsonpath="{.items[0].metadata.name}")

if [ -z "$POD" ]; then
  echo "❌ Error: Toxiproxy pod not found in namespace '$NAMESPACE'."
  exit 1
fi

echo "🐢 Injecting latency (${LATENCY_MS}ms +/- ${JITTER_MS}ms) into ${PROXY_NAME} on pod ${POD}..."

# Remove existing toxic if exists
kubectl exec -n "$NAMESPACE" "$POD" -- /toxiproxy-cli toxic remove \
  --toxicName "$TOXIC_NAME" \
  "$PROXY_NAME" >/dev/null 2>&1 || true

# Add new latency toxic
kubectl exec -n "$NAMESPACE" "$POD" -- /toxiproxy-cli toxic add \
  --toxicName "$TOXIC_NAME" \
  --type latency \
  --attribute latency="$LATENCY_MS" \
  --attribute jitter="$JITTER_MS" \
  "$PROXY_NAME"

# Verify result
echo "✅ Latency injected. Current status:"
kubectl exec -n "$NAMESPACE" "$POD" -- /toxiproxy-cli inspect "$PROXY_NAME"

