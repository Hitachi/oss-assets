import express from 'express';
import axios from 'axios';
import cors from 'cors';

const app = express();

// Configurable ports and names via env
const PORT = process.env.PORT ?? 8080;
const SERVER_NAME = process.env.SERVER_NAME ?? 'MCP-HELLO';

// PRM settings (authorization servers exposed in metadata)
const AUTHZ_SERVERS = (process.env.PRM_AUTH_SERVERS ?? '')
  .split(',')
  .filter(Boolean);

// Downstream MCP-B base URL (egress via Envoy)
const MCP_B_BASE_URL = process.env.MCP_B_BASE_URL ?? '';

// MCP protocol version (override via env if needed)
const MCP_PROTOCOL_VERSION = process.env.MCP_PROTOCOL_VERSION ?? '2024-11-05';

// CORS: keep minimal, managed centrally
const allowOrigin = process.env.CORS_ALLOW_ORIGIN ?? '*';
app.use(
  cors({
    origin: allowOrigin,
    methods: ['GET', 'POST', 'OPTIONS'],
    allowedHeaders: ['authorization', 'content-type', 'x-request-id', 'mcp-protocol-version'],
    credentials: false,
  })
);
app.use((req, res, next) => {
  res.header('Vary', 'Origin');
  next();
});

app.use(express.json());

// Public (unprotected) endpoints
app.get('/health', (req, res) => res.send('OK'));

// Protected resource example
app.get('/hello', (req, res) => {
  res.json({
    message: `Hello from ${SERVER_NAME}`,
    server: SERVER_NAME,
    path: '/hello',
    time: new Date().toISOString(),
  });
});

// PRM (Protected Resource Metadata)
app.get('/.well-known/oauth-protected-resource', (req, res) => {
  const host = req.headers['host'] ?? `localhost:${PORT}`;
  const scheme = req.headers['x-forwarded-proto'] ?? 'http';
  res.json({
    resource: `${scheme}://${host}/`,
    authorization_servers: AUTHZ_SERVERS,
    bearer_methods_supported: ['header'],
    scopes_supported: ['mcp.read', 'mcp.write'],
  });
});

// Minimal JSON Schemas for tool inputs
const helloInputSchema = {
  type: 'object',
  properties: { name: { type: 'string', description: 'Optional greeting target' } },
  required: [],
};
const callBInputSchema = {
  type: 'object',
  properties: { name: { type: 'string', description: 'Optional name passed to MCP-B hello' } },
  required: [],
};

// Call MCP-B hello via MCP protocol
async function callMcpBHello(authorization = '', name) {
  if (!MCP_B_BASE_URL) throw new Error('MCP_B_BASE_URL is not set');

  const headers = {
    authorization: authorization ?? '',
    'mcp-protocol-version': MCP_PROTOCOL_VERSION,
    'content-type': 'application/json',
  };

  // 1) initialize
  await axios.post(
    `${MCP_B_BASE_URL}/mcp`,
    {
      jsonrpc: '2.0',
      id: Date.now(),
      method: 'initialize',
      params: { clientInfo: { name: SERVER_NAME, version: '0.1.0' }, protocolVersion: MCP_PROTOCOL_VERSION },
    },
    { headers }
  );

  // 2) tools/call -> hello
  const r2 = await axios.post(
    `${MCP_B_BASE_URL}/mcp`,
    {
      jsonrpc: '2.0',
      id: Date.now() + 1,
      method: 'tools/call',
      params: { name: 'hello', arguments: name ? { name } : {} },
    },
    { headers }
  );

  return r2.data; // Pass downstream JSON-RPC response as-is
}

// MCP endpoint (JSON-RPC 2.0)
app.post('/mcp', async (req, res) => {
  res.setHeader('Content-Type', 'application/json');

  const msg = req.body ?? {};
  const id = msg.id ?? null;
  const method = msg.method;

  // Notifications (no id) -> 202 Accepted
  if (id === null && method) return res.status(202).end();

  // initialize
  if (method === 'initialize') {
    return res.status(200).json({
      jsonrpc: '2.0',
      id,
      result: {
        protocolVersion: MCP_PROTOCOL_VERSION,
        serverInfo: { name: 'mcp-hello', version: '0.1.0' },
        capabilities: { tools: { list: true, call: true } },
      },
    });
  }

  // tools/list
  if (method === 'tools/list') {
    return res.status(200).json({
      jsonrpc: '2.0',
      id,
      result: {
        tools: [
          { name: 'hello', description: 'Return a simple greeting message', inputSchema: helloInputSchema },
          { name: 'call-b', description: 'Call MCP-B hello via MCP protocol', inputSchema: callBInputSchema },
        ],
      },
    });
  }

  // tools/call
  if (method === 'tools/call') {
    try {
      const name = msg.params?.name;
      const args = msg.params?.arguments ?? {};

      if (name === 'hello') {
        const payload = {
          message: `Hello ${args.name ?? ''}`.trim(),
          server: SERVER_NAME,
          path: '/hello',
          time: new Date().toISOString(),
        };
        return res.status(200).json({
          jsonrpc: '2.0',
          id,
          result: {
            content: [
              { type: 'text', text: payload.message },
              {
                type: 'resource',
                resource: { uri: 'memory://hello', mimeType: 'application/json', text: JSON.stringify(payload) },
              },
            ],
            isError: false,
          },
        });
      }

      if (name === 'call-b') {
        const auth = req.headers['authorization'] ?? '';
        const downstream = await callMcpBHello(auth, args.name);
        return res.status(200).json({
          jsonrpc: '2.0',
          id,
          result: {
            content: [
              { type: 'text', text: `Called MCP-B hello ${args.name ?? ''}`.trim() },
              { type: 'resource', resource: { uri: 'memory://call-b', mimeType: 'application/json', text: JSON.stringify(downstream) } },
            ],
            isError: false,
          },
        });
      }

      return res.status(200).json({ jsonrpc: '2.0', id, error: { code: -32601, message: `Unknown tool: ${name}` } });
    } catch (e) {
      return res.status(200).json({ jsonrpc: '2.0', id, error: { code: -32001, message: e?.message ?? 'tool call failed' } });
    }
  }

  // unsupported method
  return res.status(200).json({ jsonrpc: '2.0', id, error: { code: -32601, message: `Unsupported method: ${method}` } });
});

app.listen(PORT, () => {
  console.log(`MCP Hello World server ${SERVER_NAME} listening on ${PORT}`);
});
