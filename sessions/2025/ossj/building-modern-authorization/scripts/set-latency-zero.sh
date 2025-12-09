#!/bin/bash

set -euo pipefail

# Configuration
NAMESPACE="common-central"
PROXY_NAME="keycloak_proxy"
TOXIC_NAME="demo_latency"

# Retrieve the Toxiproxy Pod name
POD=$(kubectl get pod -n "$NAMESPACE" -l app=toxiproxy -o jsonpath="{.items[0].metadata.name}")

if [ -z "$POD" ]; then
  echo "❌ Error: Toxiproxy pod not found in namespace '$NAMESPACE'."
  exit 1
fi

echo "🚀 Removing latency from ${PROXY_NAME} on pod ${POD}..."

# Remove the toxic
kubectl exec -n "$NAMESPACE" "$POD" -- /toxiproxy-cli toxic remove \
  --toxicName "$TOXIC_NAME" \
  "$PROXY_NAME" >/dev/null 2>&1 || true

# Verify result
echo "✅ Latency removed. Current status:"
kubectl exec -n "$NAMESPACE" "$POD" -- /toxiproxy-cli inspect "$PROXY_NAME"
