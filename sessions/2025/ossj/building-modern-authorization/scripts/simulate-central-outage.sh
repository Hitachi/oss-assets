#!/bin/bash
echo "🔥 SIMULATING CENTRAL OUTAGE..."
echo "Scaling down all control plane services to 0..."

kubectl scale deployment --all -n common-central --replicas=0

echo "⏳ Waiting for pods to terminate..."
kubectl wait --for=delete pod --all -n common-central --timeout=60s

echo "💀 Central Control Plane is DEAD."
