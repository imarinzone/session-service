# Session Service

A Go-based OAuth2 session service that provides JWT token issuance and validation, designed to integrate with AWS API Gateway.

## Features

- **OAuth2 Token Endpoint**: Supports `client_credentials` and `refresh_token` grant types
- **JWT Tokens**: RS256 signed JWT access tokens with JWKS support
- **Refresh Tokens**: Long-lived refresh tokens for token renewal
- **PostgreSQL Storage**: Client credentials stored securely with bcrypt hashing
- **Redis Caching**: Client metadata caching, rate limiting, and token revocation
- **Rate Limiting**: Per-client rate limiting with Redis
- **Token Revocation**: Support for revoking both access and refresh tokens
- **JWKS Endpoint**: Public key endpoint for JWT validation

## Architecture

```
Client → API Gateway → Session Service
                        ├── PostgreSQL (client credentials)
                        └── Redis (caching, rate limiting, tokens)
```

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Redis 6 or higher
- Docker and Docker Compose (for local development)

## Quick Start

### Using Makefile (Recommended)

The easiest way to get started is using the provided Makefile:

```bash
# Complete development setup (generates keys, starts services, runs migrations)
make dev-setup

# Or step by step:
make generate-keys      # Generate RSA key pair
make docker-up          # Start PostgreSQL and Redis
make migrate            # Run database migrations
make create-client      # Create a test client interactively
make run                # Start the service
```

See all available commands:
```bash
make help
```

### Manual Setup

#### 1. Generate RSA Keys

```bash
# Generate private key
openssl genrsa -out private.pem 2048

# Extract public key
openssl rsa -in private.pem -pubout -out public.pem

# Or use make
make generate-keys
```

#### 2. Set Environment Variables

Copy `.env.example` to `.env` and update with your configuration:

```bash
cp .env.example .env
```

Update the `JWT_PRIVATE_KEY` and `JWT_PUBLIC_KEY` with your generated keys.

#### 3. Start Services with Docker Compose

```bash
docker-compose up -d
# Or use make
make docker-up
```

This will start:
- PostgreSQL on port 5432
- Redis on port 6379
- Session Service on port 8080

#### 4. Run Database Migrations

```bash
make migrate
# Or manually:
psql $DATABASE_URL -f migrations/001_init.sql
```

#### 5. Create a Client

Use the Makefile helper:
```bash
make create-client
```

Or manually connect to PostgreSQL and insert a client:

```sql
INSERT INTO clients (client_id, client_secret_hash, rate_limit)
VALUES (
  'my-client',
  '$2a$10$...', -- bcrypt hash of your client secret
  100
);
```

To generate a bcrypt hash:

```bash
# Using Go
go run -c 'golang.org/x/crypto/bcrypt' <<< 'your-secret'

# Or using htpasswd
htpasswd -nbBC 10 "" your-secret | cut -d: -f2
```

### 5. Test the Service

#### Get Access Token (Client Credentials Grant)

```bash
curl -X POST http://localhost:8080/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=my-client&client_secret=your-secret"
```

Response:
```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "abc123..."
}
```

#### Refresh Token

```bash
curl -X POST http://localhost:8080/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=refresh_token&refresh_token=abc123..."
```

#### Verify Token

```bash
curl -X POST http://localhost:8080/oauth/verify \
  -H "Content-Type: application/json" \
  -d '{"token": "eyJ..."}'
```

#### Get JWKS

```bash
curl http://localhost:8080/.well-known/jwks.json
```

## API Endpoints

### POST /oauth/token

Issues access and refresh tokens.

**Client Credentials Grant:**
```
grant_type=client_credentials
client_id=<client_id>
client_secret=<client_secret>
```

**Refresh Token Grant:**
```
grant_type=refresh_token
refresh_token=<refresh_token>
```

### POST /oauth/verify

Validates a JWT token and returns claims if valid.

**Request:**
```json
{
  "token": "eyJ..."
}
```

**Response:**
```json
{
  "valid": true,
  "claims": {
    "iss": "session-service",
    "aud": "api",
    "sub": "client-id",
    "exp": 1234567890,
    "iat": 1234564290,
    "jti": "uuid"
  }
}
```

### GET /.well-known/jwks.json

Returns the public keys in JWKS format for JWT validation.

### GET /health

Health check endpoint.

## Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `JWT_PRIVATE_KEY` | RSA private key (PEM format) | - |
| `JWT_PUBLIC_KEY` | RSA public key (PEM format) | - |
| `JWT_ISSUER` | Token issuer claim | `session-service` |
| `JWT_AUDIENCE` | Token audience claim | `api` |
| `JWT_EXPIRY` | Access token expiration | `3600s` |
| `REFRESH_TOKEN_EXPIRY` | Refresh token expiration | `604800s` (7 days) |
| `REFRESH_TOKEN_LENGTH` | Refresh token length in bytes | `32` |
| `SERVER_PORT` | HTTP server port | `8080` |

## AWS API Gateway Integration

### JWT Authorizer Setup

1. Configure API Gateway JWT Authorizer:
   - **JWKS URI**: `https://your-service/.well-known/jwks.json`
   - **Issuer**: Value of `JWT_ISSUER` (default: `session-service`)
   - **Audience**: Value of `JWT_AUDIENCE` (default: `api`)

2. API Gateway will automatically:
   - Validate JWT signature using JWKS
   - Check token expiration
   - Verify issuer and audience claims
   - Return 401 for invalid/expired tokens

### Lambda Deployment

Package the service as a Lambda function:

```bash
GOOS=linux GOARCH=amd64 go build -o bootstrap ./cmd/server
zip function.zip bootstrap
```

## Development

### Common Makefile Commands

```bash
make help           # Show all available commands
make build          # Build the application
make run            # Run the application locally
make test           # Run tests
make test-coverage  # Run tests with coverage report
make lint           # Format code and run go vet
make clean          # Clean build artifacts
make docker-up      # Start all services
make docker-down    # Stop all services
make docker-logs    # View service logs
make migrate        # Run database migrations
```

### Run Locally

```bash
# Start dependencies
make docker-up

# Run migrations
make migrate

# Run the service
make run
# Or: go run ./cmd/server
```

### Run Tests

```bash
make test
# Or: go test ./...

# With coverage
make test-coverage
```

### Build

```bash
make build
# Or: go build -o server ./cmd/server
```

## Project Structure

```
session-service/
├── cmd/server/          # Application entry point
├── internal/
│   ├── auth/           # JWT generation and validation
│   ├── cache/          # Redis operations
│   ├── config/         # Configuration management
│   ├── database/       # PostgreSQL operations
│   ├── handlers/       # HTTP handlers
│   ├── middleware/     # HTTP middleware
│   └── models/         # Data models
├── migrations/         # Database migrations
├── pkg/errors/         # Error types
└── test/              # Unit tests
```

## Security Considerations

- Client secrets are hashed using bcrypt
- JWT tokens are signed with RS256 (asymmetric keys)
- Refresh tokens are stored securely in Redis
- Rate limiting prevents abuse
- Token revocation support
- HTTPS/TLS should be used in production

## License

MIT

