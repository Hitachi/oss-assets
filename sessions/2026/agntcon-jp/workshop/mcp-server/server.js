import "./telemetry.js";

import express from "express";
import { randomUUID } from "node:crypto";
import { trace } from "@opentelemetry/api";

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { isInitializeRequest } from "@modelcontextprotocol/sdk/types.js";

const tracer = trace.getTracer("workshop-mcp");

const app = express();
app.use(express.json());

const transports = new Map();

function createMcpServer() {
  const server = new McpServer({
    name: "workshop-mcp-server",
    version: "1.0.0"
  });

  server.tool(
    "ping",
    "Health check",
    {},
    async () =>
      tracer.startActiveSpan("tool.ping", async (span) => {
        try {
          span.setAttribute("tool.name", "ping");
          return {
            content: [
              {
                type: "text",
                text: "pong"
              }
            ]
          };
        } finally {
          span.end();
        }
      })
  );

  server.tool(
    "list_customers",
    "List customers",
    {},
    async () =>
      tracer.startActiveSpan(
        "tool.list_customers",
        async (span) => {
          try {
            span.setAttribute(
              "required.scope",
              "read"
            );

            return {
              content: [
                {
                  type: "text",
                  text: JSON.stringify(
                    [
                      {
                        id: 1001,
                        name: "Alice Corp"
                      },
                      {
                        id: 1002,
                        name: "Bob Industries"
                      }
                    ],
                    null,
                    2
                  )
                }
              ]
            };
          } finally {
            span.end();
          }
        }
      )
  );

  server.tool(
    "delete_customer",
    "Delete customer",
    {},
    async () =>
      tracer.startActiveSpan(
        "tool.delete_customer",
        async (span) => {
          try {
            span.setAttribute(
              "required.scope",
              "admin"
            );

            return {
              content: [
                {
                  type: "text",
                  text: "customer deleted"
                }
              ]
            };
          } finally {
            span.end();
          }
        }
      )
  );

  return server;
}

app.post("/mcp", async (req, res) => {
  try {
    const sessionId = req.headers["mcp-session-id"];

    let transport;

    if (sessionId && transports.has(sessionId)) {
      transport = transports.get(sessionId);
    } else if (!sessionId && isInitializeRequest(req.body)) {
      transport = new StreamableHTTPServerTransport({
        sessionIdGenerator: () => randomUUID(),

        onsessioninitialized: (newSessionId) => {
          transports.set(newSessionId, transport);
          console.log(`MCP session initialized: ${newSessionId}`);
        }
      });

      transport.onclose = () => {
        const closedSessionId = transport.sessionId;

        if (closedSessionId) {
          transports.delete(closedSessionId);
          console.log(`MCP session closed: ${closedSessionId}`);
        }
      };

      const server = createMcpServer();
      await server.connect(transport);
    } else {
      res.status(400).json({
        jsonrpc: "2.0",
        error: {
          code: -32000,
          message: "Bad Request: invalid or missing MCP session ID"
        },
        id: null
      });
      return;
    }

    await transport.handleRequest(req, res, req.body);
  } catch (error) {
    console.error("MCP POST error:", error);

    if (!res.headersSent) {
      res.status(500).json({
        jsonrpc: "2.0",
        error: {
          code: -32603,
          message: "Internal server error"
        },
        id: null
      });
    }
  }
});

app.get("/mcp", async (req, res) => {
  try {
    const sessionId = req.headers["mcp-session-id"];

    if (!sessionId || !transports.has(sessionId)) {
      res.status(400).send("Invalid or missing MCP session ID");
      return;
    }

    const transport = transports.get(sessionId);
    await transport.handleRequest(req, res);
  } catch (error) {
    console.error("MCP GET error:", error);

    if (!res.headersSent) {
      res.status(500).send("Internal server error");
    }
  }
});

app.delete("/mcp", async (req, res) => {
  try {
    const sessionId = req.headers["mcp-session-id"];

    if (!sessionId || !transports.has(sessionId)) {
      res.status(400).send("Invalid or missing MCP session ID");
      return;
    }

    const transport = transports.get(sessionId);
    await transport.handleRequest(req, res);
  } catch (error) {
    console.error("MCP DELETE error:", error);

    if (!res.headersSent) {
      res.status(500).send("Internal server error");
    }
  }
});

app.listen(3000, "0.0.0.0", () => {
  console.log("MCP Server listening on port 3000");
});