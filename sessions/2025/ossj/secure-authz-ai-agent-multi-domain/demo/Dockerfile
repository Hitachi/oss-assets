# Minimal runtime image for the demo
FROM node:22-alpine

WORKDIR /app

# Install only production deps; rely on lockfile if present
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# App sources
COPY server.js ./

ENV NODE_ENV=production
CMD ["node", "server.js"]
