# EBS SSO Gateway

A lightweight, independent OIDC-based Single Sign-On (SSO) gateway for Oracle E-Business Suite 12.2, built in Go.

## Overview

The EBS SSO Gateway enables modern OIDC authentication for Oracle EBS without requiring WebLogic or OAM. It supports multiple OIDC providers including:
- Microsoft Entra ID (formerly Azure AD)
- Okta
- Ping Identity

## Architecture

The gateway implements the standard OIDC Authorization Code Flow:
1. User accesses `/login` and is redirected to the OIDC provider
2. After authentication, provider redirects to `/callback` with authorization code
3. Gateway exchanges code for ID token and extracts user identity
4. User is looked up in EBS `FND_USER` table
5. EBS session is created using Oracle PL/SQL APIs
6. Session cookies are set and user is redirected to EBS

## Project Structure

```
ebssso/
├── main.go                 # Application entry point
├── config/
│   └── config.go          # Configuration management
├── handlers/
│   └── auth.go            # OIDC authentication handlers
├── db/
│   └── oracle.go          # Oracle database integration
├── templates/
│   ├── error.html         # Error page template
│   └── logout.html        # Logout page template
├── config.yaml.template   # Configuration template
├── go.mod                 # Go module dependencies
└── requirements.md        # Original requirements
```

## Prerequisites

- Go 1.21 or higher
- Oracle Instant Client (for godror driver)
- Access to Oracle E-Business Suite database
- OIDC provider (Entra ID, Okta, or Ping) with configured application

## Configuration

1. Copy the configuration template:
   ```bash
   cp config.yaml.template config.yaml
   ```

2. Edit `config.yaml` with your environment-specific settings:
   - OIDC provider details (URL, Client ID, Client Secret)
   - Oracle database connection string
   - EBS base URL and cookie settings
   - Logging preferences

3. Set the environment variable (optional):
   ```bash
   export EBS_ENV=DEV  # or TEST, PROD
   ```

## Building

```bash
# Download dependencies
go mod download

# Build the application
go build -o ebssso

# Build for Linux (from any platform)
GOOS=linux GOARCH=amd64 go build -o ebssso
```

## Running

```bash
# Run directly
./ebssso

# Run with specific config file
./ebssso

# The application will look for config.yaml in the current directory
# or config.{EBS_ENV}.yaml if EBS_ENV is set
```

## API Endpoints

- `GET /` - Redirects to `/login`
- `GET /login` - Initiates OIDC authentication flow
- `GET /callback` - Handles OIDC provider callback
- `GET /logout` - Terminates EBS session and clears cookies
- `GET /health` - Health check endpoint

## Environment Variables

- `EBS_ENV` - Environment name (DEV, TEST, PROD) - defaults to DEV

## Dependencies

- **gin-gonic/gin** - High-performance HTTP web framework
- **coreos/go-oidc** - OpenID Connect client implementation
- **godror/godror** - Oracle database driver for Go
- **sirupsen/logrus** - Structured logging
- **gopkg.in/yaml.v3** - YAML configuration parsing

## Security Features

- CSRF protection using state parameter
- Secure cookie handling with HttpOnly flag
- ID token verification
- Connection pooling for database
- Graceful error handling
- Structured logging for audit trail

## Deployment

### As a Linux Service

1. Copy the binary to a system directory:
   ```bash
   sudo cp ebssso /usr/local/bin/
   ```

2. Create a systemd service file `/etc/systemd/system/ebssso.service`:
   ```ini
   [Unit]
   Description=EBS SSO Gateway
   After=network.target

   [Service]
   Type=simple
   User=ebssso
   WorkingDirectory=/opt/ebssso
   ExecStart=/usr/local/bin/ebssso
   Environment="EBS_ENV=PROD"
   Restart=on-failure
   RestartSec=10

   [Install]
   WantedBy=multi-user.target
   ```

3. Enable and start the service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable ebssso
   sudo systemctl start ebssso
   ```

### Docker (Optional)

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o ebssso

FROM alpine:latest
RUN apk --no-cache add ca-certificates libaio libnsl libc6-compat
COPY --from=builder /app/ebssso /ebssso
COPY --from=builder /app/templates /templates
ENTRYPOINT ["/ebssso"]
```

## Oracle Database Requirements

The EBS database user needs the following permissions:
- `SELECT` on `FND_USER`
- `SELECT` and `UPDATE` on `ICX_SESSIONS`
- `EXECUTE` on `ICX_SEC` package

## Troubleshooting

### Database Connection Issues
- Verify Oracle Instant Client is installed
- Check `LD_LIBRARY_PATH` includes Oracle libraries
- Verify database connection string format

### OIDC Authentication Issues
- Verify redirect URL is registered with OIDC provider
- Check client ID and secret are correct
- Ensure scopes include at least `openid`, `profile`, `email`

### User Not Found
- Verify user exists in `FND_USER` table
- Check email addresses match between OIDC and EBS
- Confirm user account is not expired (`END_DATE IS NULL`)

## Logging

Logs include:
- All authentication attempts (success and failure)
- Session creation and termination
- Database operations
- HTTP request/response details

Log level can be configured in `config.yaml`: `debug`, `info`, `warn`, `error`

## Performance

- Designed for 10-200 concurrent requests
- Uses connection pooling for Oracle database
- Goroutines for concurrent request handling
- Configurable timeouts and limits

## License

[Specify your license here]

## Support

For issues and questions, please contact your system administrator or refer to the requirements documentation.
