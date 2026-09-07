package authz

import future.keywords.in

default allowed = false

claims := object.get(
    object.get(
        input.attributes.metadataContext.filterMetadata,
        "dev.agentgateway.jwt",
        {}
    ),
    "claims",
    {}
)

token_scopes := split(
    object.get(claims, "scope", ""),
    " "
)

body := object.get(input, "parsed_body", {})

mcp_method := object.get(body, "method", "")

tool_name := object.get(
    object.get(body, "params", {}),
    "name",
    ""
)

required_scopes := ["read", "admin"] if {
    tool_name == "delete_customer"
}

required_scopes := ["read"] if {
    tool_name == "list_customers"
}

required_scopes := [] if {
    tool_name == "ping"
}

missing_scopes := [
    s |
    s := required_scopes[_]
    not s in token_scopes
]

is_public_tool if {
    tool_name == "ping"
}

is_bypass_method if {
    input.attributes.request.http.method == "GET"
}

is_bypass_method if {
    mcp_method != "tools/call"
}

has_token if {
    claims != null
}

has_required_scopes if {
    every s in required_scopes {
        s in token_scopes
    }
}

is_authorized if {
    is_bypass_method
}

else if {
    is_public_tool
}

else if {
    has_required_scopes
}

#
# Permit
#
main := {
    "allowed": true
} if {
    is_authorized
}

#
# 401
#
main := {
    "allowed": false,
    "http_status": 401,
    "headers": {
        "content-type": "application/json",
        "www-authenticate": "Bearer"
    },
    "body":
        sprintf(
            "{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32001,\"message\":\"Authentication required for tool %s\"},\"id\":null}",
            [tool_name]
        )
} if {
    not is_bypass_method
    not is_public_tool
    not has_token
}

#
# 403
#
main := {
    "allowed": false,
    "http_status": 403,
    "headers": {
        "content-type": "application/json",
        "www-authenticate":
            sprintf(
                "Bearer error=\"insufficient_scope\", scope=\"%s\"",
                [concat(" ", missing_scopes)]
            )
    },
    "body":
        sprintf(
            "{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32003,\"message\":\"Missing scopes: %s for tool %s\"},\"id\":null}",
            [concat(", ", missing_scopes), tool_name]
        )
} if {
    has_token
    not is_authorized
}