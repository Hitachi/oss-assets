import logging
import webbrowser
import asyncio
import secrets
import hashlib
import base64
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs, urlencode
import threading
from keycloak.keycloak_openid import KeycloakOpenID

# Logging configuration
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Keycloak configuration
KEYCLOAK_SERVER_URL = "http://localhost:8080"
KEYCLOAK_REALM = "myrealm"
KEYCLOAK_CLIENT_ID = "mcp"
KEYCLOAK_CLIENT_SECRET = "<secret>"
REDIRECT_URI = "http://localhost:8888/callback"
REQUESTED_SCOPE = "hello"

# Global variables
auth_code = None
code_verifier = None
current_tokens = {
    "access_token": None,
    "refresh_token": None,
}


async def login_flow():
    """Login using authorization code flow with HTTP Basic Authentication"""
    global auth_code, code_verifier
    
    auth_code = None
    
    logger.info("=" * 80)
    logger.info("Starting Keycloak Authorization Code Flow")
    logger.info("Client Authentication Method: HTTP Basic (OAuth 2.1 recommended)")
    logger.info("=" * 80)
    
    # Generate PKCE parameters
    code_verifier = base64.urlsafe_b64encode(secrets.token_bytes(32)).decode('utf-8').rstrip('=')
    code_challenge = base64.urlsafe_b64encode(
        hashlib.sha256(code_verifier.encode('utf-8')).digest()
    ).decode('utf-8').rstrip('=')
    
    logger.info(f"Code Verifier: {code_verifier}")
    logger.info(f"Code Challenge: {code_challenge}")
    logger.info("Code Challenge Method: S256")
    
    # Create authorization URL
    auth_params = {
        "client_id": KEYCLOAK_CLIENT_ID,
        "redirect_uri": REDIRECT_URI,
        "response_type": "code",
        "scope": REQUESTED_SCOPE,
        "state": "random_state",
        "code_challenge": code_challenge,
        "code_challenge_method": "S256"
    }
    auth_url = f"{KEYCLOAK_SERVER_URL}/realms/{KEYCLOAK_REALM}/protocol/openid-connect/auth?{urlencode(auth_params)}"
    
    logger.info(f"Authorization URL: {auth_url}")
    logger.info("=" * 80)
    
    # Start callback server
    logger.info("Starting callback server (localhost:8888)...")
    server = start_callback_server()
    await asyncio.sleep(1)
    
    # Open authorization URL in browser
    logger.info("Opening authentication page in browser...")
    webbrowser.open(auth_url)
    
    # Wait for authorization code
    logger.info("Waiting for authorization code...")
    for i in range(120):
        await asyncio.sleep(1)
        if auth_code:
            break
        if i % 10 == 9:
            logger.info(f"Waiting... ({i+1} seconds elapsed)")
    
    if not auth_code:
        logger.error("✗ Timeout: Failed to receive authorization code")
        return False
    
    # Exchange authorization code for access token using HTTP Basic Authentication
    try:
        logger.info("=" * 80)
        logger.info("Requesting token with HTTP Basic Authentication...")
        logger.info(f"Using Code Verifier: {code_verifier}")
        
        # Create a separate KeycloakOpenID instance for HTTP Basic authentication
        keycloak_openid = KeycloakOpenID(
            server_url=KEYCLOAK_SERVER_URL,
            client_id=KEYCLOAK_CLIENT_ID,
            realm_name=KEYCLOAK_REALM,
            client_secret_key=None  # Don't set secret to avoid body inclusion
        )
        
        # Create HTTP Basic Authentication header
        credentials = f"{KEYCLOAK_CLIENT_ID}:{KEYCLOAK_CLIENT_SECRET}"
        basic_auth = base64.b64encode(credentials.encode()).decode()
        
        # Add Authorization header with Basic authentication
        keycloak_openid.connection.add_param_headers("Authorization", f"Basic {basic_auth}")
                
        # Call token endpoint
        token_response = keycloak_openid.token(
            grant_type='authorization_code',
            code=auth_code,
            redirect_uri=REDIRECT_URI,
            code_verifier=code_verifier
        )
        
        # Save token information
        current_tokens["access_token"] = token_response.get("access_token")
        current_tokens["refresh_token"] = token_response.get("refresh_token")
        
        logger.info("✓ Access token obtained successfully!")
        logger.info(f"Token type: {token_response.get('token_type')}")
        logger.info(f"Expires in: {token_response.get('expires_in')} seconds")
        logger.info(f"Scope: {token_response.get('scope')}")
        logger.info(f"Refresh token: {'Available' if current_tokens['refresh_token'] else 'Not available'}")
        logger.info(f"Access token: {current_tokens['access_token']}")
        logger.info("=" * 80)
        
        return True
        
    except Exception as e:
        logger.error(f"✗ Token request error: {e}")
        import traceback
        logger.error(traceback.format_exc())
        return False
    
    
def start_callback_server():
    """Start callback server"""
    server = HTTPServer(('localhost', 8888), CallbackHandler)
    server_thread = threading.Thread(target=server.handle_request)
    server_thread.daemon = True
    server_thread.start()
    return server


class CallbackHandler(BaseHTTPRequestHandler):
    """HTTP handler to receive authorization code"""
    
    def log_message(self, format, *args):
        """Suppress default logs"""
        pass
    
    def do_GET(self):
        global auth_code
        
        parsed_path = urlparse(self.path)
        query_params = parse_qs(parsed_path.query)
        
        logger.info("=" * 80)
        logger.info("Authorization response received")
        
        if 'code' in query_params:
            auth_code = query_params['code'][0]
            logger.info(f"✓ Authorization code received: {auth_code}")
            
            self.send_response(200)
            self.send_header('Content-type', 'text/html; charset=utf-8')
            self.end_headers()
            html = """
            <html>
            <head><title>Authentication Successful</title></head>
            <body>
                <h1>Authentication Successful!</h1>
                <p>You can close this window and return to the terminal.</p>
            </body>
            </html>
            """
            self.wfile.write(html.encode('utf-8'))
        
        elif 'error' in query_params:
            error = query_params['error'][0]
            error_description = query_params.get('error_description', [''])[0]
            logger.error(f"✗ Authorization error: {error} - {error_description}")
            
            self.send_response(400)
            self.send_header('Content-type', 'text/html; charset=utf-8')
            self.end_headers()
            html = f"<h1>Authentication Error</h1><p>{error}: {error_description}</p>"
            self.wfile.write(html.encode('utf-8'))
        
        logger.info("=" * 80)


async def main():
    try:
        success = await login_flow()
        if success:
            print("\n" + "=" * 80)
            print("Program completed successfully")
            print("=" * 80)
        else:
            print("\n" + "=" * 80)
            print("Failed to obtain token")
            print("=" * 80)
    except Exception as e:
        logger.error(f"An error occurred: {e}")
        print("\n" + "=" * 80)
        print("Program terminated with error")
        print("=" * 80)


if __name__ == "__main__":
    asyncio.run(main())
