#!/bin/bash
set -e

cd "$(dirname "$0")/.."

echo "🚀 Starting OSSJ Demo Environment Setup..."

# --- 0. Permission Fix ---
echo "🛠️  Setting executable permissions for scripts..."
chmod +x scripts/*.sh

# --- 1. Namespaces ---
echo "📦 Creating Namespaces..."
kubectl apply -f 00-common-central/namespace.yaml
kubectl apply -f 01-centralized-tenant/namespace.yaml
kubectl apply -f 02-distributed-tenant/namespace.yaml
kubectl apply -f 03-hybrid-tenant/namespace.yaml
kubectl apply -f 10-common-app/namespace.yaml

# --- 2. Common Central (Infrastructure Only) ---
echo "🔑 Deploying Common Central Infrastructure..."
kubectl apply -f 00-common-central/keycloak/
kubectl apply -f 00-common-central/toxiproxy/
kubectl apply -f 00-common-central/gitea/

# --- 3. Gitea Setup (Critical Step) ---
echo "⏳ Waiting for Gitea to be ready..."
kubectl wait --for=condition=ready pod -l app=gitea -n common-central --timeout=120s
echo "✅ Gitea is up!"

echo "⚙️  Configuring Gitea (User & Repository)..."
./scripts/setup-gitea-user.sh
./scripts/setup-gitea-repo.sh

# --- 4. OPAL Server & CronJob ---
echo "📡 Deploying OPAL Server & Sync Job..."
kubectl apply -f 00-common-central/opal-server/
kubectl apply -f 00-common-central/keycloak2gitea/

# --- 5. Tenant Environments ---
echo "🏗️  Deploying Tenant Environments..."
echo "  👉 01-centralized"
kubectl apply -f 01-centralized-tenant/ --recursive
echo "  👉 02-distributed"
kubectl apply -f 02-distributed-tenant/ --recursive
echo "  👉 03-hybrid"
kubectl apply -f 03-hybrid-tenant/ --recursive

# --- 6. Demo UI ---
echo "🖥️  Deploying Demo Frontend..."
kubectl apply -f 10-common-app/frontend/

# --- 7. Finalize ---
echo "---------------------------------------------------"
echo "✅ All manifests applied successfully!"
echo ""

echo "⏳ Waiting for Keycloak setup job to complete..."
kubectl wait --for=condition=complete job/keycloak-setup -n common-central --timeout=300s

echo "🐢 Injecting Initial Network Latency..."
kubectl wait --for=condition=ready pod -l app=toxiproxy -n common-central --timeout=60s
./scripts/set-latency-high.sh

echo "🎉 Environment Setup Complete!"
echo "---------------------------------------------------"
echo "You can now access the Demo UI:"
echo "👉 Run: ./scripts/start-tunnels.sh"
echo "👉 Open: http://localhost:30000"
