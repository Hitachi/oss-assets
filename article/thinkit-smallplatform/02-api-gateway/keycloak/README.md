# Keycloak

## Keycloakデプロイ

```sh
helm upgrade --install keycloak oci://registry-1.docker.io/bitnamicharts/keycloak \
--set auth.adminUser=admin \
--set auth.adminPassword=admin \
-n keycloak --create-namespace
```

## HTTPRouteデプロイ

```sh
kubectl apply -f httproute.yaml
```

## AuthPolicyデプロイ

```sh
kubectl apply -f authpolicy-allow-all.yaml
```
