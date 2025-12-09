#!/bin/bash
set -euo pipefail

# Configuration
GITEA_LOCAL_PORT=30004
USER="opal"
PASS="password"
REPO="policies"
API_URL="http://localhost:$GITEA_LOCAL_PORT/api/v1"

echo "🚀 Setting up Gitea with Final Demo Configuration..."

# 0. Cleanup ports
lsof -ti:$GITEA_LOCAL_PORT | xargs kill -9 >/dev/null 2>&1 || true

# 1. Start Port-Forward
echo "🔌 Starting port-forward to Gitea..."
kubectl port-forward svc/gitea -n common-central $GITEA_LOCAL_PORT:3000 > /dev/null 2>&1 &
PID=$!

echo "⏳ Waiting for Gitea API..."
for i in {1..10}; do
    if curl -s "http://localhost:$GITEA_LOCAL_PORT/api/v1/version" > /dev/null; then
        echo "✅ Connected."
        break
    fi
    sleep 1
done

# --- 1.5 Delete Repository if exists ---
echo "🗑️  Deleting existing repository..."
curl -s -X DELETE "$API_URL/repos/$USER/$REPO" \
  -u "$USER:$PASS" \
  -H "Content-Type: application/json" > /dev/null

# --- 2. Create Repository ---
echo "📦 Creating repository '$REPO'..."
curl -s -o /dev/null -X POST "$API_URL/user/repos" \
  -u "$USER:$PASS" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"$REPO\", \"private\": false, \"auto_init\": true, \"default_branch\": \"main\"}"

# --- Function to upload file ---
upload_file() {
    local file_path=$1
    local content=$2
    local message=$3
    
    local b64_content=$(echo "$content" | openssl base64 -A)
    local payload="{
        \"content\": \"$b64_content\",
        \"message\": \"$message\",
        \"branch\": \"main\"
    }"

    local http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API_URL/repos/$USER/$REPO/contents/$file_path" \
      -u "$USER:$PASS" \
      -H "Content-Type: application/json" \
      -d "$payload")
      
    if [ "$http_code" -eq 201 ]; then
        echo "✅ Created: $file_path"
    else
        echo "❌ Failed: $file_path (HTTP $http_code)"
    fi
}

# ==========================================
#  Structure: Distributed (Local Data)
# ==========================================

# 1. distributed/.manifest
DIST_MANIFEST=$(cat <<EOF
policy.rego
data.json
EOF
)
upload_file "distributed/.manifest" "$DIST_MANIFEST" "Init Distributed manifest"

# 2. distributed/data.json
DIST_DATA=$(cat <<EOF
{
  "permissions": {
    "alice": [ { "role": "admin" } ],
    "bob": [ { "role": "guest" } ]
  }
}
EOF
)
upload_file "distributed/data.json" "$DIST_DATA" "Init Distributed data"

# 3. distributed/policy.rego
DIST_REGO=$(cat <<EOF
package envoy.authz

import future.keywords.if

default allow = false

# Main Logic
allow if {
    input.attributes.request.http.method == "GET"
    check_group_logic
}

# 1. Group A Company -> Allow immediately
check_group_logic if {
    has_group("A Company")
}

# 2. Group B Company -> Check Role from Local Data
check_group_logic if {
    has_group("B Company")
    has_role("admin")
}

# --- Helpers ---
has_group(group_name) if {
    groups := token_payload.groups
    some g in groups
    g == group_name
}

has_role(role_name) if {
    username := token_payload.preferred_username
    # Check Local Data (distributed/data.json -> data.permissions)
    user_perms := data.distributed.permissions[username]
    some i
    user_perms[i].role == role_name
}

token_payload := payload if {
    auth_header := input.attributes.request.http.headers.authorization
    startswith(auth_header, "Bearer ")
    token_str := substring(auth_header, 7, -1)
    [_, payload, _] := io.jwt.decode(token_str)
}
EOF
)
upload_file "distributed/policy.rego" "$DIST_REGO" "Init Distributed policy"


# ==========================================
#  Structure: Hybrid (Real-time Fetch)
# ==========================================

# 1. hybrid/.manifest
HYBRID_MANIFEST=$(cat <<EOF
policy.rego
EOF
)
upload_file "hybrid/.manifest" "$HYBRID_MANIFEST" "Init Hybrid manifest"

# 2. hybrid/policy.rego
HYBRID_REGO=$(cat <<EOF
package envoy.authz

import future.keywords.if

default allow = false

# Decord JWT
jwt_payload := payload if {
  auth := input.attributes.request.http.headers.authorization
  parts := split(auth, " ")
  count(parts) == 2
  token := parts[1]
  [_, payload, _] := io.jwt.decode(token)
}

# Main Logic
allow if {
  in_group("A Company")
}

allow if {
  in_group("B Company")
  user := kc_user_from_jwt
  user.attributes.role[_] == "admin"
}

# --- Helpers ---
in_group(name) if {
  payload := jwt_payload
  payload.groups[_] == name
}

kc_user_from_jwt = user if {
  payload := jwt_payload
  sub := payload.sub
  user := kc_user(sub)
}

admin_token := token if {
  KC_URL := "http://toxiproxy.common-central.svc.cluster.local:8080"
  REALM  := "master"
  USER   := "admin"
  PASS   := "admin"
  CLIENT := "admin-cli"

  url := sprintf("%s/realms/%s/protocol/openid-connect/token", [KC_URL, REALM])

  body := sprintf(
    "client_id=%s&username=%s&password=%s&grant_type=%s",
    [CLIENT, USER, PASS, "password"]
  )

  resp := http.send({
    "method": "POST",
    "url": url,
    "headers": {
      "Content-Type": "application/x-www-form-urlencoded"
    },
    "raw_body": body,
    "raise_error": false,
    "timeout": "5s"
  })

  resp.status_code == 200
  token := resp.body.access_token
}

kc_user(sub) = user if {
  KC_URL := "http://toxiproxy.common-central.svc.cluster.local:8080"
  REALM  := "demo"
  token  := admin_token

  url := sprintf("%s/admin/realms/%s/users/%s", [KC_URL, REALM, sub])

  resp := http.send({
    "method": "GET",
    "url": url,
    "headers": {
      "Authorization": sprintf("Bearer %s", [token])
    },
    "raise_error": false,
    "timeout": "5s"
  })

  resp.status_code == 200
  user := resp.body
}
EOF
)
upload_file "hybrid/policy.rego" "$HYBRID_REGO" "Init Hybrid policy"


# --- Cleanup ---
echo "🔌 Stopping port-forward..."
kill $PID >/dev/null 2>&1 || true

echo "🔄 Restarting OPAL Server to enforce fresh clone..."
kubectl delete pod -l app=opal-server -n common-central

echo "🎉 Repo setup complete!"
