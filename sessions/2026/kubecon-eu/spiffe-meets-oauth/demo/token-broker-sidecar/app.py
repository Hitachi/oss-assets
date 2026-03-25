from fastapi import FastAPI, Request, Response
from fastapi.responses import PlainTextResponse
from urllib.parse import urlparse
from spiffe import WorkloadApiClient
import os, httpx
import logging
import time

app = FastAPI()

# Config via environment (see kc-broker-a secret)
KC_A_TOKEN_URL      = os.getenv("KC_A_TOKEN_URL", "")
KC_A_ISSUER_URL = KC_A_TOKEN_URL.split("/protocol/openid-connect/token")[0]
KC_A_SCOPE          = os.getenv("KC_A_SCOPE", "")
KC_CLIENT_ID_FOR_A = os.getenv("KC_CLIENT_ID_FOR_A", "")
KC_CLIENT_SECRET_FOR_A = os.getenv("KC_CLIENT_SECRET_FOR_A", "")
CLIENT_AUTH_METHOD_FOR_A = os.getenv("CLIENT_AUTH_METHOD_FOR_A", "client_secret_post")
KC_B_TOKEN_URL      = os.getenv("KC_B_TOKEN_URL", "")
KC_B_ISSUER_URL = KC_B_TOKEN_URL.split("/protocol/openid-connect/token")[0]
KC_CLIENT_ID_FOR_B = os.getenv("KC_CLIENT_ID_FOR_B", "")
KC_CLIENT_SECRET_FOR_B = os.getenv("KC_CLIENT_SECRET_FOR_B", "")
CLIENT_AUTH_METHOD_FOR_B = os.getenv("CLIENT_AUTH_METHOD_FOR_B", "client_secret_post")
TIMEOUT_MS          = int(os.getenv("HTTP_TIMEOUT_MS", "1000"))  # default 1s
LOG_LEVEL = os.getenv("LOG_LEVEL", "DEBUG").upper()

logging.basicConfig(
    level=getattr(logging, LOG_LEVEL, logging.INFO),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("kc-broker")

def get_jwt_svid(audience: str, spiffe_id: str) -> str:
    with WorkloadApiClient() as c:
        svid = c.fetch_jwt_svid(audience={audience}, subject=spiffe_id)
        return svid.token

@app.on_event("startup")
async def _startup_log():
    logger.info(
        "startup KC_A_TOKEN_URL=%s KC_B_TOKEN_URL=%s TIMEOUT_MS=%d LOG_LEVEL=%s",
        KC_A_TOKEN_URL, KC_B_TOKEN_URL, TIMEOUT_MS, LOG_LEVEL
    )

@app.get("/healthz")
async def healthz():
    return {"status": "ok"}

# ExtAuthz contract: 200 allow, 401 deny. Return Authorization header on success.
@app.get("/{rest:path}")
@app.post("/{rest:path}")
async def broker(req: Request):
    start = time.time()
    # 1) Extract incoming access token (Bearer). Required.
    auth = req.headers.get("authorization", "")
    if not auth.lower().startswith("bearer "):
        logger.info("deny missing_bearer method=%s path=%s", req.method, req.url.path)
        return PlainTextResponse("", status_code=401)
    incoming = auth.split(" ", 1)[1].strip()
    rid = req.headers.get("x-request-id", "")
    logger.debug(
        "request method=%s path=%s x-request-id=%s incoming=%s",
        req.method, req.url.path, rid, incoming
    )

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

            headers_a = {
                "Content-Type": "application/x-www-form-urlencoded",
            }

            if CLIENT_AUTH_METHOD_FOR_A == "client_secret_post":
                data_a["client_id"] = KC_CLIENT_ID_FOR_A
                data_a["client_secret"] = KC_CLIENT_SECRET_FOR_A
            elif CLIENT_AUTH_METHOD_FOR_A == "spiffe":
                jwt_svid_a = get_jwt_svid(KC_A_ISSUER_URL, KC_CLIENT_ID_FOR_A)
                data_a["client_assertion_type"] = "urn:ietf:params:oauth:client-assertion-type:jwt-spiffe"
                data_a["client_assertion"] = jwt_svid_a

            r_a = await client.post(KC_A_TOKEN_URL, data=data_a, headers=headers_a)
            if r_a.status_code >= 500:
                logger.warning(
                    "deny step=A upstream_5xx status=%s ct=%s method=%s path=%s x-request-id=%s",
                    r_a.status_code, r_a, req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=503)
            
            if r_a.status_code >= 400:
                logger.info(
                    "deny step=A upstream_4xx status=%s ct=%s body=%s method=%s path=%s x-request-id=%s",
                    r_a.status_code, r_a, (r_a.text or "")[:200],
                    req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=401)
            try:
                body_a = r_a.json()
            except Exception:
                logger.info(
                    "deny step=A nonjson status=%s ct=%s body=%s method=%s path=%s x-request-id=%s",
                    r_a.status_code, r_a, (r_a.text or "")[:200],
                    req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=401)

            # Access token (JWT) issued for audience B
            jwt_for_b = body_a.get("access_token", "")
            if not jwt_for_b:                
                logger.info(
                    "deny step=A missing_access_token method=%s path=%s x-request-id=%s",
                    req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=401)

            # 4) Step-B: JWT Authorization Grant at realm-B
            # grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer
            data_b = {
                "grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
                "assertion": jwt_for_b,
            }

            headers_b = {
                "Content-Type": "application/x-www-form-urlencoded",
            }

            if CLIENT_AUTH_METHOD_FOR_B == "client_secret_post":
                data_b["client_id"] = KC_CLIENT_ID_FOR_B
                data_b["client_secret"] = KC_CLIENT_SECRET_FOR_B
            elif CLIENT_AUTH_METHOD_FOR_B == "spiffe":
                jwt_svid_b = get_jwt_svid(KC_B_ISSUER_URL, KC_CLIENT_ID_FOR_B)
                data_b["client_assertion_type"] = "urn:ietf:params:oauth:client-assertion-type:jwt-spiffe"
                data_b["client_assertion"] = jwt_svid_b

            r_b = await client.post(KC_B_TOKEN_URL, data=data_b, headers=headers_b)
            if r_b.status_code >= 500:
                logger.warning(
                    "deny step=B upstream_5xx status=%s ct=%s method=%s path=%s x-request-id=%s",
                    r_b.status_code, r_b, req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=503)
            
            if r_b.status_code >= 400:
                logger.info(
                    "deny step=B upstream_4xx status=%s ct=%s body=%s method=%s path=%s x-request-id=%s",
                    r_b.status_code, r_b, (r_b.text or "")[:200],
                    req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=401)
            try:
                body_b = r_b.json()
            except Exception:
                logger.info(
                    "deny step=B nonjson status=%s ct=%s body=%s method=%s path=%s x-request-id=%s",
                    r_b.status_code, r_b, (r_b.text or "")[:200],
                    req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=401)

            # Final token to be injected upstream
            final_token = body_b.get("access_token", "")
            if not final_token:                
                logger.info(
                    "deny step=B missing_access_token method=%s path=%s x-request-id=%s",
                    req.method, req.url.path, rid
                )
                return PlainTextResponse("", status_code=401)

    except Exception as e:
        logger.warning(
            "deny exception method=%s path=%s x-request-id=%s err=%s",
            req.method, req.url.path, rid, repr(e),
            exc_info=True,
        )
        # If you want to keep the old behavior (always 401), change this back to 401.
        return PlainTextResponse("", status_code=503)

    # 5) Allow: inject Authorization for upstream request (Envoy will pass this header)
    headers_out = {
        "Authorization": f"Bearer {final_token}",
        # Optional: hint proxies not to cache auth result
        "Cache-Control": "no-store",
    }    
    logger.info(
        "allow method=%s path=%s x-request-id=%s elapsed_ms=%d",
        req.method, req.url.path, rid, int((time.time() - start) * 1000)
    )
    return Response(content="", status_code=200, headers=headers_out)
