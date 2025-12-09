#!/bin/bash
set -euo pipefail

# --- Configuration ---
PORT_UI=30000
PORT_KC=8080
PORT_GIT=30004

# --- Cleanup Function ---
cleanup() {
    echo ""
    echo "🔌 Stopping all port-forwards..."
    # Kill all background jobs started by this script
    kill $(jobs -p) 2>/dev/null || true
    echo "✅ All tunnels stopped."
}

# Trap Ctrl+C (SIGINT) and Exit to run cleanup
trap cleanup EXIT INT

echo "🚀 Starting Port-Forwarding for OSSJ Demo..."

# --- 1. Pre-flight Cleanup ---
# Kill any existing processes on these ports to avoid conflicts
echo "🧹 Cleaning up old connections..."
lsof -ti:$PORT_UI,$PORT_KC,$PORT_GIT | xargs kill -9 >/dev/null 2>&1 || true

# --- 2. Start Tunnels ---

# Frontend UI
echo "🌐 Forwarding Demo UI..."
kubectl port-forward svc/frontend -n common-app $PORT_UI:80 > /dev/null 2>&1 &
PID_UI=$!

# Keycloak
echo "🔑 Forwarding Keycloak..."
kubectl port-forward svc/keycloak -n common-central $PORT_KC:8080 > /dev/null 2>&1 &
PID_KC=$!

# Gitea
echo "📦 Forwarding Gitea..."
kubectl port-forward svc/gitea -n common-central $PORT_GIT:3000 > /dev/null 2>&1 &
PID_GIT=$!

# --- 3. Status Report ---
sleep 2 # Wait a bit for connections to establish

echo ""
echo "✅ Tunnels Established! Access your environment at:"
echo "---------------------------------------------------"
echo "🖥️  Demo UI:    http://localhost:$PORT_UI"
echo "🔐 Keycloak:   http://localhost:$PORT_KC/admin/"
echo "🐙 Gitea:      http://localhost:$PORT_GIT"
echo "---------------------------------------------------"
echo "📝 (Press Ctrl+C to stop all tunnels)"

# Keep script running to maintain tunnels
wait
