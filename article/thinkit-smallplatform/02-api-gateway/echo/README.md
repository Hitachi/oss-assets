# echo-basic API

echo-basic APIはAPIプラットフォームにデプロイするサンプルアプリケーションです。

## echo-basic APIのデプロイ

Namespaceを作成します。

```bash
kubectl create namespace echo
```

echo-basic APIをデプロイします。

```bash
kubectl apply -f echo-basic.yaml
```

HTTPRouteをデプロイします。

```bash
kubectl apply -f httproute.yaml
```
