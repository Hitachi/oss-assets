FROM node:22-bookworm

WORKDIR /app

COPY package.json .
RUN npm config set strict-ssl false && npm install

COPY server.js .
COPY telemetry.js .

EXPOSE 3000

CMD ["node", "server.js"]
