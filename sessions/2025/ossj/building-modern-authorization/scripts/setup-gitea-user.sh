#!/bin/bash

# Get Gitea Pod name
POD=$(kubectl get pod -n common-central -l app=gitea -o jsonpath="{.items[0].metadata.name}")

echo "Found Gitea pod: $POD"
echo "Creating admin user 'opal'..."

# Create admin user via Gitea CLI inside the container
# --admin: Grant admin privileges
# --username opal / --password password
# --email opal@example.com
kubectl exec -n common-central $POD -- /usr/local/bin/gitea admin user create --admin --username opal --password password --email opal@example.com

echo "✅ Admin user 'opal' created!"
