# Kubernetesへのデプロイ

${{values.name}}をKubernetesにデプロイするためのKustomize設定方法を説明します。

## ディレクトリ構造

```
kustomize/
├── base/                      # ベース設定
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── httproute.yaml
│   └── kustomization.yaml
└── overlays/
    └── production/            # 本番環境設定
```

## 前提条件

- kustomize
- Gateway API がクラスターにインストール済み
- `shared-gateway` が `shared-gateway` ネームスペースに存在（echoセクション設定済み）

## マニフェスト確認

```bash
kustomize build kustomize/overlays/production
```

## デプロイ

### 事前準備

#### イメージプルシークレット

本番環境にデプロイする前に、イメージプルシークレットを作成する必要があります：

```bash
kubectl create secret docker-registry regcred \
  -n ${{values.k8sResource.namespace}} \
  --docker-server=${{values.image.registry}} \
  --docker-username=<username> \
  --docker-password=<personal-access-token>
```

#### トークンイントロスペクションのシークレット作成

クライアントを作成する。

Client ID: `token-introspection`
Client authentication: On
Authentication method: なし (デフォルトでStandard flowが付与されるので、Offにする)

クライアントシークレットを取得し、以下のコマンドでKubernetesシークレットを作成する：

```bash
kubectl create secret generic keycloak-${{values.k8sResource.name}}-token-introspection-client-secret \
-n kuadrant-system \
--from-literal=clientID=token-introspection \
--from-literal=clientSecret=<client-secret>
```

#### クライアント作成

以下の設定でクライアントを作成する。

Client ID: `${{values.name}}`
Client authentication: On
Authentication method: Service account roles (デフォルトでStandard flowが付与されるので、Offにする)

クライアントをメモしておく。

### アプリケーションデプロイ

```bash
kustomize build kustomize/overlays/production | kubectl apply -f -
```

## 確認

```bash
kubectl get pods,svc,httproute,hpa -n ${{values.k8sResource.namespace}}
```

## アンデプロイ

```bash
kustomize build kustomize/overlays/production | kubectl delete -f -
```

## 動作確認

```bash
CLIENT_ID=${{values.name}}
CLIENT_SECRET=<Keycloakから取得した${{values.name}}クライアントのクライアントシークレット>
ACCESS_TOKEN=$(curl -X POST "http://keycloak.172.32.4.127.nip.io/realms/${{values.keycloakRealm}}/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=$CLIENT_ID" \
  -d "client_secret=$CLIENT_SECRET" | jq -r '.access_token')
```

```bash
curl -i -H "Authorization: Bearer $ACCESS_TOKEN" \
http://${{values.domain}}/hello
```
