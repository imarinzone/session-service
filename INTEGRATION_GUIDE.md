# Server-to-Server Authentication Integration Guide (BFF Pattern)

This guide outlines how to integrate the `session-service` (Auth Server) into a server-to-server authentication flow using the **Backend for Frontend (BFF)** pattern. This approach enhances security by:

- Keeping the long-lived `refresh_token` stored securely on the Backend Server.
- Exposing only short-lived `access_token` tokens to the Frontend.
- Ensuring PII (name, phone, email) stays in your backend DB while JWTs carry only opaque IDs + roles/scopes.

## Overview

The authentication flow relies on:

- The **OAuth 2.0 Client Credentials Grant** for the initial token acquisition, where the BFF sends trusted user + tenant context.
- The **Refresh Token Grant** for renewing expired tokens.

> **Important:** All API endpoints (except `/.well-known/openid-configuration`) are **tenant-scoped**, meaning they require `tenant_id` as part of the URL path. For example:
> - Token endpoint: `POST /{tenant_id}/oauth2/v2.0/token`
> - JWKS endpoint: `GET /{tenant_id}/discovery/v1.0/keys`
> - Verify endpoint: `POST /{tenant_id}/oauth2/v1.0/verify`
> - Health endpoint: `GET /{tenant_id}/health`

### Actors

*   **User**: The end-user interacting with the Frontend.
*   **Frontend**: The client application (e.g., Web App, Mobile App).
*   **Backend Server 1 (BFF)**: The service handling the initial user action, performing authentication, and **securely storing the refresh token**.
*   **Auth Server**: This service (`session-service`), responsible for issuing tokens.
*   **Backend Server 2**: The resource server that the Frontend wants to access.

## Sequence Diagram

```mermaid
sequenceDiagram
    actor User
    participant Frontend
    participant Backend1 as Backend Server 1 (BFF)
    participant AuthServer as Auth Server (session-service)
    participant Backend2 as Backend Server 2

    Note over User, Backend1: Initial Authentication Flow
    User->>Frontend: Clicks Icon / Action
    Frontend->>Backend1: Request Action
    Backend1->>AuthServer: POST /{tenant_id}/oauth2/v2.0/token (client_credentials)
    Note right of Backend1: tenant_id in path<br/>client_id, client_secret in body
    AuthServer-->>Backend1: 200 OK (Access Token, Refresh Token)
    Note right of Backend1: Store Refresh Token Securely<br/>(e.g., Encrypted DB / Session)
    Backend1-->>Frontend: Response (Access Token ONLY)

    Note over Frontend, Backend2: Protected Resource Access
    Frontend->>Backend2: Request Resource (Authorization: Bearer <token>)
    Backend2->>AuthServer: GET /{tenant_id}/discovery/v1.0/keys (Optional)
    Note right of Backend2: Validate token using JWKS<br/>or POST /{tenant_id}/oauth2/v1.0/verify
    Backend2-->>Frontend: Resource Data

    Note over Frontend, Backend1: Token Refresh Flow
    Frontend->>Frontend: Check Token Expiry
    alt Token Expired
        Frontend->>Backend1: Request New Token (Session Cookie / ID)
        Note right of Backend1: Retrieve Stored Refresh Token<br/>& Tenant ID
        Backend1->>AuthServer: POST /{tenant_id}/oauth2/v2.0/token (refresh_token)
        AuthServer-->>Backend1: 200 OK (New Access Token, New Refresh Token)
        Note right of Backend1: Update Stored Refresh Token
        Backend1-->>Frontend: New Access Token
    end
```

## Integration Steps

### 1. Initial Authentication (Backend 1 -> Auth Server)

When the user triggers the action, **Backend Server 1** must authenticate with the **Auth Server** to obtain tokens. The flow supports two scenarios:

#### 1a. First-Time User Login (User Provisioning)

For users that don't exist in the Auth Server's database yet, you must provide user details to create the user record.

*   **Endpoint**: `POST /{tenant_id}/oauth2/v2.0/token`
*   **Content-Type**: `application/x-www-form-urlencoded`
*   **Path Parameters**:
    *   `tenant_id` (required): Internal tenant ID in your system (must already exist in the `tenants` table). This is included in the URL path.
*   **Body Parameters** (Client Credentials Grant - First-Time Login):
    *   `grant_type`: `client_credentials`
    *   `client_id`: Your Client ID
    *   `client_secret`: Your Client Secret
    *   `user_id` (required): Internal user ID in your system (opaque, stable).
    *   `user_full_name` (required for first-time): Full name of the user (stored in DB, **never** placed in JWT).
    *   `user_phone` (required for first-time): Phone number of the user (stored in DB, **never** placed in JWT).
    *   `user_email` (optional): Email of the user (stored in DB, **never** placed in JWT).
    *   `user_roles` (optional): Comma-separated list of roles for this user within the tenant (e.g., `tenant-admin,reader`).

**Example Request (First-Time Login):**

```bash
curl -X POST http://auth-server:8080/tenant-abc/oauth2/v2.0/token \
     -d "grant_type=client_credentials" \
     -d "client_id=YOUR_CLIENT_ID" \
     -d "client_secret=YOUR_CLIENT_SECRET" \
     -d "user_id=user-123" \
     -d "user_full_name=Jane Doe" \
     -d "user_phone=+15555551234" \
     -d "user_email=jane@example.com" \
     -d "user_roles=tenant-admin,reader"
```

#### 1b. Subsequent User Login (User Already Exists)

For users that already exist in the Auth Server's database, you only need to provide the `user_id`. The Auth Server will verify the user exists, fetch their roles from the database, and issue tokens.

*   **Endpoint**: `POST /{tenant_id}/oauth2/v2.0/token`
*   **Content-Type**: `application/x-www-form-urlencoded`
*   **Path Parameters**:
    *   `tenant_id` (required): Internal tenant ID in your system (must already exist in the `tenants` table). This is included in the URL path.
*   **Body Parameters** (Client Credentials Grant - Subsequent Login):
    *   `grant_type`: `client_credentials`
    *   `client_id`: Your Client ID
    *   `client_secret`: Your Client Secret
    *   `user_id` (required): Internal user ID in your system (opaque, stable).

**Example Request (Subsequent Login):**

```bash
curl -X POST http://auth-server:8080/tenant-abc/oauth2/v2.0/token \
     -d "grant_type=client_credentials" \
     -d "client_id=YOUR_CLIENT_ID" \
     -d "client_secret=YOUR_CLIENT_SECRET" \
     -d "user_id=user-123"
```

> **Note:** 
> - The `tenant_id` is part of the URL path (`/{tenant_id}/oauth2/v2.0/token`), not a form parameter. This makes the API explicitly tenant-scoped.
> - For first-time login, if `user_full_name` or `user_phone` are missing, the request will be rejected with an `INVALID_REQUEST` error.
> - For subsequent logins, if the user doesn't exist or belongs to a different tenant, the request will be rejected.
> - User roles are fetched from the database for subsequent logins, so role changes will be reflected automatically.

**Example Response (Internal to Backend 1):**

```json
{
    "access_token": "eyJhbGciOiJIUzI1Ni...",
    "token_type": "Bearer",
    "expires_in": 3600,
    "refresh_token": "def50200..."
}
```

### 2. Secure Storage & Response to Frontend

**Backend Server 1** must:
1.  **Store the `refresh_token` securely**. This could be in a server-side session store (Redis, DB) linked to the user's session ID, or an encrypted HTTP-Only cookie.
2.  **Send ONLY the `access_token`** (and expiry) to the **Frontend**.

**Response to Frontend:**

```json
{
    "access_token": "eyJhbGciOiJIUzI1Ni...",
    "expires_in": 3600
}
```

### 3. Accessing Protected Resources (Frontend -> Backend 2)

The **Frontend** uses the `access_token` to make authenticated requests to **Backend Server 2**.

*   **Header**: `Authorization: Bearer <access_token>`

**Example Request:**

```bash
curl -X GET http://backend-server-2/api/resource \
     -H "Authorization: Bearer eyJhbGciOiJIUzI1Ni..."
```

### 4. Token Refresh (Frontend -> Backend 1 -> Auth Server)

The **Frontend** monitors the token's expiration. When expired, it requests a new token from **Backend Server 1**.

**Step 4a: Frontend requests refresh from Backend 1**

Frontend calls a refresh endpoint on Backend 1. It does **NOT** send a refresh token (it doesn't have one). It sends its session identifier (e.g., cookie).

**Step 4b: Backend 1 calls Auth Server**

Backend 1 retrieves the stored `refresh_token` and `tenant_id` associated with the user's session.

*   **Endpoint**: `POST /{tenant_id}/oauth2/v2.0/token`
*   **Content-Type**: `application/x-www-form-urlencoded`
*   **Path Parameters**:
    *   `tenant_id` (required): The tenant ID from the stored refresh token subject.
*   **Body Parameters**:
    *   `grant_type`: `refresh_token`
    *   `refresh_token`: The stored refresh token.

> **Important:** The `tenant_id` in the URL path **must match** the `tenant_id` stored in the refresh token's subject. If they don't match, the request will be rejected with an `INVALID_REFRESH_TOKEN` error.

**Example Request:**

```bash
curl -X POST http://auth-server:8080/tenant-abc/oauth2/v2.0/token \
     -d "grant_type=refresh_token" \
     -d "refresh_token=STORED_REFRESH_TOKEN"
```

**Step 4c: Backend 1 updates storage and returns new Access Token**

1.  Backend 1 receives the new `access_token` and `refresh_token`.
2.  Backend 1 **updates the stored refresh token** (Refresh Token Rotation).
3.  Backend 1 returns the **new `access_token`** to the Frontend.

> **Note on Subject Consistency:**  
> Refresh tokens carry the original **user + tenant subject** (`user_id`, `tenant_id`, roles, scopes). The refreshed access token uses the **same subject**, so your authorization layer in **Backend Server 2** will see a consistent identity across refresh cycles.

## Trusted User & Tenant Provisioning

The `client_credentials` flow in this service supports **two modes** of operation:

### First-Time User Provisioning

When a user doesn't exist in the Auth Server's database:

- **Required Fields**: `user_id`, `user_full_name`, `user_phone`
- **Optional Fields**: `user_email`, `user_roles`
- **Behavior**: The Auth Server creates a new user record and stores the provided information.
- **Use Case**: Initial user registration or when syncing a new user from your BFF's user database.

### Subsequent User Authentication

When a user already exists in the Auth Server's database:

- **Required Fields**: `user_id` only
- **Behavior**: 
  - The Auth Server verifies the user exists and belongs to the specified tenant.
  - User roles are automatically fetched from the database.
  - No user data updates occur (user details remain unchanged).
- **Use Case**: Regular token requests for existing users.

### Tenant & User Management

- **Tenants**
  - `tenant_id` must already exist in the `tenants` table (created via migrations or your own admin tooling).
  - `tenant_id` is part of the URL path (`/{tenant_id}/oauth2/v2.0/token`), making the API explicitly tenant-scoped.
  - Calls with unknown `tenant_id` in the path are rejected with an `INVALID_REQUEST`-style error.

- **Users**
  - PII (`full_name`, `phone_number`, `email`) is **only stored in the DB**, never in the JWT.
  - User records are keyed by `user_id` (unique across all tenants).
  - Each user must belong to exactly one tenant (enforced by foreign key).

- **Roles**
  - Roles are stored in a `user_roles` table and fetched automatically for existing users.
  - For first-time login, `user_roles` (comma-separated) can be provided to set initial roles.
  - Role updates require a separate API call or admin operation (not updated during subsequent token requests).

This design allows your BFF to:

- Keep its **own canonical user/tenant model**.
- Use the Auth Server as a **projection** that issues **minimal, opaque, JWTs** that only contain IDs + authorization context.
- Minimize data transfer for subsequent token requests (only `user_id` needed).

## Token Claims & Multi-Tenancy

Access tokens issued by this service are **user + tenant scoped**; there are no “client-only” access tokens.

Key JWT claims:

- `sub`: User ID (`user_id`).
- `oid`: Same as `sub` (user object ID / stable user identifier).
- `tid`: Tenant ID (`tenant_id`).
- `roles`: Array of roles for the user in that tenant (from `user_roles`).
- `scp`: Array of scopes (if configured for your client flow).

**Backend Server 2** should:

- Treat `(tid, sub)` as the primary identity tuple.
- Use `roles` and/or `scp` to authorize actions.
- Never assume PII is available in the token; look it up from your own DB if needed.

## API Reference

All endpoints listed below are tenant-scoped and require `tenant_id` in the URL path. The only exception is `/.well-known/openid-configuration`, which is used for OIDC discovery and does not require tenant scoping.

### `POST /{tenant_id}/oauth2/v2.0/token`

Generates access and refresh tokens. This endpoint is **tenant-scoped**, meaning the `tenant_id` is part of the URL path.

**Path Parameters:**

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `tenant_id` | string | Yes | Internal tenant ID (must already exist in the `tenants` table). For `refresh_token` grant, must match the `tenant_id` in the refresh token's subject. |

**Body Parameters:**

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `grant_type` | string | Yes | `client_credentials` or `refresh_token` |
| `client_id` | string | Conditional | Required for `client_credentials` |
| `client_secret` | string | Conditional | Required for `client_credentials` |
| `user_id` | string | Conditional | Required for `client_credentials` (user + tenant-scoped tokens only) |
| `user_full_name` | string | Conditional | Required for `client_credentials` **only if user doesn't exist** (first-time login); stored in DB only |
| `user_phone` | string | Conditional | Required for `client_credentials` **only if user doesn't exist** (first-time login); stored in DB only |
| `user_email` | string | Optional | Optional for `client_credentials` (first-time login only); stored in DB only |
| `user_roles` | string | Optional | Optional for `client_credentials` (first-time login only); comma-separated roles |
| `refresh_token` | string | Conditional | Required for `refresh_token` |

> **Note on User Fields:**
> - For **first-time login** (user doesn't exist): `user_full_name` and `user_phone` are required. `user_email` and `user_roles` are optional.
> - For **subsequent login** (user exists): Only `user_id` is required. User details and roles are fetched from the database automatically.

### `GET /{tenant_id}/discovery/v1.0/keys`

Returns the JSON Web Key Set (JWKS) containing the public keys used to verify the access tokens. **Backend Server 2** should use this endpoint to validate the tokens.

**Path Parameters:**

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `tenant_id` | string | Yes | Internal tenant ID (must already exist in the `tenants` table). |

Key management details:

- The Auth Server maintains **multiple active signing keys** (current + previous) to support **key rotation**.
- Each JWT includes a `kid` header pointing to the specific key used to sign it.
- The JWKS response may contain **multiple keys**; verifiers should:
  - Select the key by matching `kid` from the token header.
  - Cache keys for a reasonable duration.
  - Be prepared for keys to rotate periodically (controlled by `KEY_ROTATION_DAYS` and `KEY_GRACE_DAYS` environment variables).

### `POST /{tenant_id}/oauth2/v1.0/verify` (Optional)

An endpoint to verify the token validity directly with the Auth Server.

**Path Parameters:**

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `tenant_id` | string | Yes | Internal tenant ID. Must match the `tenant_id` (`tid` claim) in the token being verified. |

**Body Parameters:**

*   **Body**: `{"token": "..."}`

> **Note:** The `tenant_id` in the URL path must match the `tid` claim in the JWT token. If they don't match, the verification will fail.
