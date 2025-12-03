# Server-to-Server Authentication Integration Guide (BFF Pattern)

This guide outlines how to integrate the `session-service` (Auth Server) into a server-to-server authentication flow using the **Backend for Frontend (BFF)** pattern. This approach enhances security by keeping the long-lived `refresh_token` stored securely on the Backend Server, exposing only the short-lived `access_token` to the Frontend.

## Overview

The authentication flow relies on the **OAuth 2.0 Client Credentials Grant** for the initial token acquisition and the **Refresh Token Grant** for renewing expired tokens.

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
    Backend1->>AuthServer: POST /oauth2/v1.0/token (client_credentials)
    Note right of Backend1: client_id, client_secret
    AuthServer-->>Backend1: 200 OK (Access Token, Refresh Token)
    Note right of Backend1: Store Refresh Token Securely<br/>(e.g., Encrypted DB / Session)
    Backend1-->>Frontend: Response (Access Token ONLY)

    Note over Frontend, Backend2: Protected Resource Access
    Frontend->>Backend2: Request Resource (Authorization: Bearer <token>)
    Backend2->>AuthServer: (Optional) Verify Token / Check Keys
    Backend2-->>Frontend: Resource Data

    Note over Frontend, Backend1: Token Refresh Flow
    Frontend->>Frontend: Check Token Expiry
    alt Token Expired
        Frontend->>Backend1: Request New Token (Session Cookie / ID)
        Note right of Backend1: Retrieve Stored Refresh Token
        Backend1->>AuthServer: POST /oauth2/v1.0/token (refresh_token)
        AuthServer-->>Backend1: 200 OK (New Access Token, New Refresh Token)
        Note right of Backend1: Update Stored Refresh Token
        Backend1-->>Frontend: New Access Token
    end
```

## Integration Steps

### 1. Initial Authentication (Backend 1 -> Auth Server)

When the user triggers the action, **Backend Server 1** must authenticate with the **Auth Server** to obtain tokens.

*   **Endpoint**: `POST /oauth2/v1.0/token`
*   **Content-Type**: `application/x-www-form-urlencoded`
*   **Parameters**:
    *   `grant_type`: `client_credentials`
    *   `client_id`: Your Client ID
    *   `client_secret`: Your Client Secret

**Example Request:**

```bash
curl -X POST http://auth-server:8080/oauth2/v1.0/token \
     -d "grant_type=client_credentials" \
     -d "client_id=YOUR_CLIENT_ID" \
     -d "client_secret=YOUR_CLIENT_SECRET"
```

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

Backend 1 retrieves the stored `refresh_token` associated with the user's session.

*   **Endpoint**: `POST /oauth2/v1.0/token`
*   **Content-Type**: `application/x-www-form-urlencoded`
*   **Parameters**:
    *   `grant_type`: `refresh_token`
    *   `refresh_token`: The stored refresh token.

**Example Request:**

```bash
curl -X POST http://auth-server:8080/oauth2/v1.0/token \
     -d "grant_type=refresh_token" \
     -d "refresh_token=STORED_REFRESH_TOKEN"
```

**Step 4c: Backend 1 updates storage and returns new Access Token**

1.  Backend 1 receives the new `access_token` and `refresh_token`.
2.  Backend 1 **updates the stored refresh token** (Refresh Token Rotation).
3.  Backend 1 returns the **new `access_token`** to the Frontend.

## API Reference

### `POST /oauth2/v1.0/token`

Generates access and refresh tokens.

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `grant_type` | string | Yes | `client_credentials` or `refresh_token` |
| `client_id` | string | Conditional | Required for `client_credentials` |
| `client_secret` | string | Conditional | Required for `client_credentials` |
| `refresh_token` | string | Conditional | Required for `refresh_token` |

### `GET /discovery/v1.0/keys`

Returns the JSON Web Key Set (JWKS) containing the public keys used to verify the access tokens. **Backend Server 2** should use this endpoint to validate the tokens.

### `POST /oauth2/v1.0/verify` (Optional)

An endpoint to verify the token validity directly with the Auth Server.

*   **Body**: `{"token": "..."}`
