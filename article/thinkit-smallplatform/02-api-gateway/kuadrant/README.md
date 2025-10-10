# Kuadrant

## cert-managerインストール

cert-managerはKuadrant Operatorのインストールに必要なコンポーネントです。以下のコマンドでcert-managerをインストールします。

```bash
helm upgrade --install \
cert-manager cert-manager \
--repo https://charts.jetstack.io \
--namespace cert-manager \
--create-namespace \
--version v1.15.3 \
--set crds.enabled=true
```

## Kuadrant Operatorインストール

Kuadrant Operatorをインストールします。

```bash
helm upgrade --install \
kuadrant-operator kuadrant-operator \
--repo https://kuadrant.io/helm-charts/ \
--create-namespace \
--namespace kuadrant-system
```

## Kuadrantインストール

Kuadrantをインストールします。

```bash
kubectl apply -f kuadrant.yaml
```
