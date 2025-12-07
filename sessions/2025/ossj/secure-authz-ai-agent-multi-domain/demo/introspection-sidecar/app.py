from fastapi import FastAPI, Request, Response
from fastapi.responses import PlainTextResponse
import os, base64, httpx

app = FastAPI()

# Config via environment
KC_INTROSPECT_URL = os.getenv("KC_INTROSPECT_URL", "")
KC_CLIENT_ID = os.getenv("KC_CLIENT_ID", "")
KC_CLIENT_SECRET = os.getenv("KC_CLIENT_SECRET", "")
TIMEOUT_MS = int(os.getenv("HTTP_TIMEOUT_MS", "1000"))  # default 1s

def _basic_auth_header(cid: str, secret: str) -> str:
    raw = f"{cid}:{secret}".encode("utf-8")
    return "Basic " + base64.b64encode(raw).decode("utf-8")

@app.get("/healthz")
async def healthz():
    return {"status": "ok"}

# ExtAuthz contract: 200 allow, 401 deny, body can be empty
@app.get("/{rest:path}")
@app.post("/{rest:path}")
async def check(req: Request):
    # Authorization header required
    auth = req.headers.get("authorization", "")
    if not auth.lower().startswith("bearer "):
        return PlainTextResponse("", status_code=401)
    token = auth.split(" ", 1)[1].strip()

    # Config check
    if not (KC_INTROSPECT_URL and KC_CLIENT_ID and KC_CLIENT_SECRET):
        return PlainTextResponse("misconfigured", status_code=500)

    # RFC 7662 token introspection
    headers = {
        "Authorization": _basic_auth_header(KC_CLIENT_ID, KC_CLIENT_SECRET),
        "Content-Type": "application/x-www-form-urlencoded",
    }
    data = {"token": token, "token_type_hint": "access_token"}

    try:
        timeout = httpx.Timeout(TIMEOUT_MS / 1000.0)
        async with httpx.AsyncClient(timeout=timeout) as client:
            resp = await client.post(KC_INTROSPECT_URL, headers=headers, data=data)
    except Exception:
        return PlainTextResponse("", status_code=401)

    if resp.status_code >= 500:
        return PlainTextResponse("", status_code=401)

    try:
        body = resp.json()
    except Exception:
        return PlainTextResponse("", status_code=401)

    if not bool(body.get("active", False)):
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
    return Response(content="", status_code=200, headers=headers_out)

