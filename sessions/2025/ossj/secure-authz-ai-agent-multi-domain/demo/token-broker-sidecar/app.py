from fastapi import FastAPI, Request, Response
from fastapi.responses import PlainTextResponse
import os, base64, httpx

app = FastAPI()

# Config via environment (see kc-broker-a secret)
KC_A_TOKEN_URL      = os.getenv("KC_A_TOKEN_URL", "")
KC_A_CLIENT_ID      = os.getenv("KC_A_CLIENT_ID", "")
KC_A_CLIENT_SECRET  = os.getenv("KC_A_CLIENT_SECRET", "")
KC_A_SCOPE          = os.getenv("KC_A_SCOPE", "")  # e.g., "mcp-b"

KC_B_TOKEN_URL      = os.getenv("KC_B_TOKEN_URL", "")
KC_B_CLIENT_ID      = os.getenv("KC_B_CLIENT_ID", "")
KC_B_CLIENT_SECRET  = os.getenv("KC_B_CLIENT_SECRET", "")
KC_B_CLIENT_AUTH    = os.getenv("KC_B_CLIENT_AUTH", "client_secret_basic")  # or client_secret_post / private_key_jwt

TIMEOUT_MS          = int(os.getenv("HTTP_TIMEOUT_MS", "1000"))  # default 1s

def _client_auth_fields(client_id: str, client_secret: str, method: str):
    """
    Return auth fields for form body when using client_secret_post.
    For basic auth, caller should use HTTP basic instead (not implemented here to keep minimal).
    """
    if method == "client_secret_post":
        return {"client_id": client_id, "client_secret": client_secret}
    return {}  # client_secret_basic (recommended in this minimal sample)

def _basic_auth_header(cid: str, secret: str) -> str:
    raw = f"{cid}:{secret}".encode("utf-8")
    return "Basic " + base64.b64encode(raw).decode("utf-8")

@app.get("/healthz")
async def healthz():
    return {"status": "ok"}

# ExtAuthz contract: 200 allow, 401 deny. Return Authorization header on success.
@app.get("/{rest:path}")
@app.post("/{rest:path}")
async def broker(req: Request):
    # 1) Extract incoming access token (Bearer). Required.
    auth = req.headers.get("authorization", "")
    if not auth.lower().startswith("bearer "):
        return PlainTextResponse("", status_code=401)
    incoming = auth.split(" ", 1)[1].strip()

    # 2) Config check
    if not (KC_A_TOKEN_URL and KC_A_CLIENT_ID and KC_A_CLIENT_SECRET and KC_B_TOKEN_URL and KC_B_CLIENT_ID and KC_B_CLIENT_SECRET):
        return PlainTextResponse("misconfigured", status_code=500)

    try:
        timeout = httpx.Timeout(TIMEOUT_MS / 1000.0)
        async with httpx.AsyncClient(timeout=timeout) as client:
            # 3) Step-A: Token Exchange at realm-A (RFC 8693)
            # grant_type=urn:ietf:params:oauth:grant-type:token-exchange
            data_a = {
                "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
                "subject_token": incoming,
                "subject_token_type": "urn:ietf:params:oauth:token-type:access_token",
            }
            if KC_A_SCOPE:
                data_a["scope"] = KC_A_SCOPE

            # Use client_secret_basic by default
            # Basic auth with A-client (recommended minimal)
            headers_a = {
                "Authorization": _basic_auth_header(KC_A_CLIENT_ID, KC_A_CLIENT_SECRET),
                "Content-Type": "application/x-www-form-urlencoded",
            }

            r_a = await client.post(KC_A_TOKEN_URL, data=data_a, headers=headers_a)
            if r_a.status_code >= 500:
                return PlainTextResponse("", status_code=401)
            body_a = r_a.json()
            # Access token (JWT) issued for audience B
            jwt_for_b = body_a.get("access_token", "")
            if not jwt_for_b:
                return PlainTextResponse("", status_code=401)

            # 4) Step-B: JWT Authorization Grant at realm-B
            # grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer
            data_b = {
                "grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
                "assertion": jwt_for_b,
            }

            headers_b = {}
            if KC_B_CLIENT_AUTH == "client_secret_basic":
                headers_b = {
                    "Authorization": _basic_auth_header(KC_B_CLIENT_ID, KC_B_CLIENT_SECRET),
                    "Content-Type": "application/x-www-form-urlencoded",
                }
            else:
                headers_b = {
                    "Content-Type": "application/x-www-form-urlencoded",
                }
                data_b.update(_client_auth_fields(KC_B_CLIENT_ID, KC_B_CLIENT_SECRET, "client_secret_post"))

            r_b = await client.post(KC_B_TOKEN_URL, data=data_b, headers=headers_b)
            if r_b.status_code >= 500:
                return PlainTextResponse("", status_code=401)
            body_b = r_b.json()
            # Final token to be injected upstream
            final_token = body_b.get("access_token", "")
            if not final_token:
                return PlainTextResponse("", status_code=401)

    except Exception as e:
        print(f"[broker] exception: {e}\n")
        return PlainTextResponse("", status_code=401)

    # 5) Allow: inject Authorization for upstream request (Envoy will pass this header)
    headers_out = {
        "Authorization": f"Bearer {final_token}",
        # Optional: hint proxies not to cache auth result
        "Cache-Control": "no-store",
    }
    return Response(content="", status_code=200, headers=headers_out)
