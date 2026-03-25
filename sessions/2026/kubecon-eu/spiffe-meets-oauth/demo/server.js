import express from 'express';
import axios from 'axios';
import cors from 'cors';

const app = express();

// Configurable ports and names via env
const PORT = process.env.PORT ?? 8080;
const SERVER_NAME = process.env.SERVER_NAME ?? 'HELLO-API';

// Downstream API-B base URL
const API_B_BASE_URL = process.env.API_B_BASE_URL ?? '';

// CORS: keep minimal, managed centrally
const allowOrigin = process.env.CORS_ALLOW_ORIGIN ?? '*';
app.use(
  cors({
    origin: allowOrigin,
    methods: ['GET', 'OPTIONS'],
    allowedHeaders: ['authorization', 'content-type', 'x-request-id'],
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

// GET /hello (self)
app.get('/hello', (req, res) => {
  res.json({
    message: `Hello from ${SERVER_NAME}`,
    server: SERVER_NAME,
    path: '/hello',
    time: new Date().toISOString(),
  });
});

// GET /call-b -> call downstream API-B /hello
app.get('/call-b', async (req, res) => {
  try {
    if (!API_B_BASE_URL) {
      return res.status(500).json({ error: 'API_B_BASE_URL is not set' });
    }

    const r = await axios.get(`${API_B_BASE_URL}/hello`, {
      headers: {
        authorization: req.headers['authorization'] ?? '',
      },
      timeout: 5000,
    });

    return res.json({
      upstream: SERVER_NAME,
      downstream: 'api-b',
      response: r.data,
    });
  } catch (e) {
    return res.status(502).json({
      error: 'failed_to_call_api_b',
      message: e?.message,
    });
  }
});

app.listen(PORT, () => {
  console.log(`Hello API server ${SERVER_NAME} listening on ${PORT}`);
});
