FROM ghcr.io/modelcontextprotocol/inspector:latest

USER root

RUN npm config set strict-ssl false && \
    npm install -g \
        @opentelemetry/sdk-node \
        @opentelemetry/auto-instrumentations-node \
        @opentelemetry/exporter-trace-otlp-http

COPY telemetry.cjs /otel/telemetry.cjs

ENV NODE_PATH=/usr/local/lib/node_modules
ENV NODE_OPTIONS="--require /otel/telemetry.cjs"
