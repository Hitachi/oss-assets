# Envoy Gateway

## Envoy Gatewayインストール

Envoy Gatewayをインストールします。

```sh
helm upgrade --install eg oci://docker.io/envoyproxy/gateway-helm \
--version v1.4.1 \
--set config.envoyGateway.extensionApis.enableEnvoyPatchPolicy=true \
-n envoy-gateway-system --create-namespace
```

## GatewayClassのデプロイ

Envoy GatewayのGatewayClassをデプロイします。

```sh
kubectl apply -f gateway-class.yaml
```
