# Keycloak

## Keycloakデプロイ

Namespace作成

```sh
kubectl create namespace keycloak
```

Keycloakデプロイ

```sh
kubectl apply -f keycloak.yaml
```

## HTTPRouteデプロイ

```sh
kubectl apply -f httproute.yaml
```

## AuthPolicyデプロイ

```sh
kubectl apply -f authpolicy-allow-all.yaml
```
