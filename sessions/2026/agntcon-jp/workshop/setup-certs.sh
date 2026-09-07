#!/bin/bash

mkdir -p certs

openssl genrsa -out certs/ca.key 4096

openssl req \
  -x509 \
  -new \
  -nodes \
  -key certs/ca.key \
  -sha256 \
  -days 3650 \
  -out certs/ca.crt \
  -subj "/CN=Workshop CA"

openssl genrsa -out certs/tls.key 2048

openssl req \
  -new \
  -key certs/tls.key \
  -out certs/tls.csr \
  -subj "/CN=keycloak.localtest.me"

cat > certs/san.ext <<EOF
subjectAltName=DNS:keycloak.localtest.me
EOF

openssl x509 \
  -req \
  -in certs/tls.csr \
  -CA certs/ca.crt \
  -CAkey certs/ca.key \
  -CAcreateserial \
  -out certs/tls.crt \
  -days 3650 \
  -sha256 \
  -extfile certs/san.ext

mkdir -p grafana/data tempo/data
chmod -R 777 grafana/data tempo/data
