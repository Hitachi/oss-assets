from fastmcp import FastMCP
import logging

logger = logging.getLogger(__name__)

mcp = FastMCP(
    "Hello",
    host="localhost",
    port=3000,
    )

@mcp.tool()
def hello():
    return f"Hello."

if __name__ == "__main__":
    mcp.run(transport="streamable-http")