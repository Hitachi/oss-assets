import contextlib

from typing import List
from mcp.server.fastmcp import FastMCP
from starlette.applications import Starlette
from starlette.routing import Mount
from starlette.middleware import Middleware

from mcpauth import MCPAuth
from mcpauth.config import AuthServerType
from mcpauth.exceptions import (
    MCPAuthBearerAuthException,
    BearerAuthExceptionCode,
)
from mcpauth.types import ResourceServerConfig, ResourceServerMetadata
from mcpauth.utils import fetch_server_config
import logging

logger = logging.getLogger(__name__)

mcp = FastMCP("Hello")

auth_issuer = "http://localhost:8080/realms/myrealm"
auth_server_config = fetch_server_config(auth_issuer, AuthServerType.OIDC)
resource_id = "http://localhost:3001/mcp"
mcp_auth = MCPAuth(
    protected_resources=[
        ResourceServerConfig(
            metadata=ResourceServerMetadata(
                resource=resource_id,
                authorization_servers=[auth_server_config],
                scopes_supported=[
                    "hello",
                ],
            )
        )
    ]
)

def has_required_scopes(user_scopes: List[str], required_scopes: List[str]) -> bool:
    return all(scope in user_scopes for scope in required_scopes)

@mcp.tool()
def hello():
    auth_info = mcp_auth.auth_info
    user_scopes = auth_info.scopes if auth_info else []
    if not has_required_scopes(user_scopes, ["hello"]):
        raise MCPAuthBearerAuthException(BearerAuthExceptionCode.MISSING_REQUIRED_SCOPES)   
    return f"Hello."

@contextlib.asynccontextmanager
async def lifespan(app: Starlette):
    async with contextlib.AsyncExitStack() as stack:
        await stack.enter_async_context(mcp.session_manager.run())
        yield

bearer_auth = Middleware(mcp_auth.bearer_auth_middleware('jwt', resource=resource_id))
app = Starlette(
    routes=[
        *mcp_auth.resource_metadata_router().routes,
        Mount("/", app=mcp.streamable_http_app(), middleware=[bearer_auth]),
    ],
    lifespan=lifespan,
)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="localhost", port=3001)