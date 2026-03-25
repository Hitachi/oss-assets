from fastapi import FastAPI, Request, Response
from fastapi.responses import PlainTextResponse
from urllib.parse import urlparse
from spiffe import WorkloadApiClient
import os, httpx
import logging
import time

app = FastAPI()

# Config via environment
KC_INTROSPECT_URL = os.getenv("KC_INTROSPECT_URL", "")
KC_TOKEN_URL = os.getenv("KC_TOKEN_URL", "")
KC_CLIENT_ID = os.getenv("KC_CLIENT_ID", "")
KC_CLIENT_SECRET = os.getenv("KC_CLIENT_SECRET", "")
CLIENT_AUTH_METHOD = os.getenv("CLIENT_AUTH_METHOD", "client_secret_post")
KC_ISSUER_URL = KC_TOKEN_URL.split("/protocol/openid-connect/token")[0]
TIMEOUT_MS = int(os.getenv("HTTP_TIMEOUT_MS", "1000"))  # default 1s
LOG_LEVEL = os.getenv("LOG_LEVEL", "DEBUG").upper()

logging.basicConfig(
    level=getattr(logging, LOG_LEVEL, logging.INFO),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("introspection-sidecar")

def get_jwt_svid(audience: str, spiffe_id: str) -> str:
    with WorkloadApiClient() as c:
        svid = c.fetch_jwt_svid(audience={audience}, subject=spiffe_id)
        return svid.token

@app.get("/healthz")
async def healthz():
    return {"status": "ok"}

# ExtAuthz contract: 200 allow, 401 deny, body can be empty
@app.get("/{rest:path}")
@app.post("/{rest:path}")
async def check(req: Request):
    start = time.time()
    # Authorization header required
    auth = req.headers.get("authorization", "")
    if not auth.lower().startswith("bearer "):
        logger.info("deny missing_bearer method=%s path=%s", req.method, req.url.path)
        return PlainTextResponse("", status_code=401)
    token = auth.split(" ", 1)[1].strip()
    rid = req.headers.get("x-request-id", "")
    logger.debug(
        "request method=%s path=%s x-request-id=%s token=%s",
        req.method, req.url.path, rid, token
    )

    # RFC 7662 token introspection
    headers = {
        "Content-Type": "application/x-www-form-urlencoded",
    }
    data = {"token": token, "token_type_hint": "access_token"}
    jwt_svid = ""
    if CLIENT_AUTH_METHOD == "client_secret_post":
        data["client_id"] = KC_CLIENT_ID
        data["client_secret"] = KC_CLIENT_SECRET
    elif CLIENT_AUTH_METHOD == "spiffe":
        jwt_svid = get_jwt_svid(KC_ISSUER_URL, KC_CLIENT_ID)
        data["client_assertion_type"] = "urn:ietf:params:oauth:client-assertion-type:jwt-spiffe"
        data["client_assertion"] = jwt_svid

    try:
        timeout = httpx.Timeout(TIMEOUT_MS / 1000.0)
        async with httpx.AsyncClient(timeout=timeout) as client:
            resp = await client.post(KC_INTROSPECT_URL, headers=headers, data=data)
    except Exception as e:
        logger.warning(
            "deny introspect_exception method=%s path=%s x-request-id=%s err=%s",
            req.method, req.url.path, rid, repr(e),
            exc_info=True,
        )
        return PlainTextResponse("", status_code=503)

    if resp.status_code >= 500:        
        logger.warning(
            "deny introspect_5xx status=%s ct=%s method=%s path=%s x-request-id=%s",
            resp.status_code, resp.headers.get("content-type", ""),
            req.method, req.url.path, rid,
        )
        return PlainTextResponse("", status_code=503)

    if resp.status_code >= 400:
        logger.info(
            "deny introspect_4xx status=%s ct=%s body=%s method=%s path=%s x-request-id=%s jwt-svid=%s",
            resp.status_code, resp.headers.get("content-type", ""),
            (resp.text or "")[:200],
            req.method, req.url.path, rid, jwt_svid,
        )
        return PlainTextResponse("", status_code=401)

    try:
        body = resp.json()
    except Exception:
        logger.info(
            "deny introspect_nonjson status=%s ct=%s body=%s method=%s path=%s x-request-id=%s",
            resp.status_code, resp.headers.get("content-type", ""),
            (resp.text or "")[:200],
            req.method, req.url.path, rid,
        )
        return PlainTextResponse("", status_code=401)

    if not bool(body.get("active", False)):        
        logger.info(
            "deny inactive sub=%s scope=%s exp=%s method=%s path=%s x-request-id=%s elapsed_ms=%d",
            body.get("sub") or body.get("username") or "",
            body.get("scope") or "",
            body.get("exp") or "",
            req.method, req.url.path, rid,
            int((time.time() - start) * 1000),
        )
        return PlainTextResponse("", status_code=401)

    # Allowed: pass minimal user context upstream
    subject = body.get("sub") or body.get("username") or ""
    scope = body.get("scope") or ""
    exp = str(body.get("exp") or "")
    headers_out = {
        "X-Subject": subject,
        "X-Scope": scope,
        "X-Token-Exp": exp,
    }
    logger.info(
        "allow sub=%s scope=%s exp=%s method=%s path=%s x-request-id=%s elapsed_ms=%d",
        subject, scope, exp,
        req.method, req.url.path, rid,
        int((time.time() - start) * 1000),
    )
    return Response(content="", status_code=200, headers=headers_out)

@app.on_event("startup")
async def _startup_log():
    logger.info("startup KC_INTROSPECT_URL=%s TIMEOUT_MS=%d LOG_LEVEL=%s",
                KC_INTROSPECT_URL, TIMEOUT_MS, LOG_LEVEL)
