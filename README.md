# EBS SSO Gateway

A lightweight, independent OIDC-based Single Sign-On (SSO) gateway for Oracle E-Business Suite 12.2, built in Go.

## Features

The EBS SSO Gateway enables modern OIDC authentication for Oracle EBS without requiring WebLogic or OAM. It supports:

### Authentication & SSO
- Multiple OIDC providers (Entra ID, Okta, Ping Identity)
- CSRF-protected authentication flow
- Secure session management with EBS
- Automatic cookie injection for EBS sessions

### EBS Integration Methods
1. **Direct Database Connection** (Legacy)
   - Direct SQL access to APPS schema
   - Simple setup for development

2. **DBC File Method** (Recommended)
   - Uses EBS DBC files for secure connection
   - GUEST_USER_PWD authentication
   - Trusted node architecture
   - APPS_CONNECT UMX role integration
   - No direct APPS schema password needed

### WebADI and Advanced Features
- **WebADI Support**: Full Excel-based data integration
- **Deep Link Preservation**: Direct access to EBS forms and functions
- **Request Proxying**: Session-aware proxy for authenticated EBS requests
- **Return URL Handling**: Preserves destination during authentication
- **Path Whitelisting**: Configurable security for proxied requests
- **File Upload Support**: Handles large WebADI uploads (configurable size limits)

### Supported EBS Access Methods
- Web-based self-service pages (/OA_HTML/*)
- WebADI desktop integrator (/webadi/*)
- Servlets and JSP requests (/servlets/*)
- Oracle Forms access (/forms/*)
- Direct function and responsibility links

For detailed information about WebADI, DBC files, and trusted nodes, see [WEBADI_GUIDE.md](WEBADI_GUIDE.md).

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

### Quick Start Configuration

1. Copy the configuration template:
   ```bash
   cp config.yaml.template config.yaml
   ```

2. Choose your connection method:

   **Option A: DBC File Method (Recommended for Production)**
   ```yaml
   database:
     use_dbc: true
     dbc_file: "/path/to/secure/appsweb.dbc"
     apps_user: "SYSADMIN"
   ```

   **Option B: Direct Connection (Development/Legacy)**
   ```yaml
   database:
     use_dbc: false
     username: "apps"
     password: "apps_password"
     connection_string: "localhost:1521/EBSDB"
   ```

3. Configure OIDC provider:
   ```yaml
   oidc:
     provider_url: "https://login.microsoftonline.com/{tenant}/v2.0"
     client_id: "your-client-id"
     client_secret: "your-client-secret"
   ```

4. Enable WebADI support (optional):
   ```yaml
   ebs:
     enable_proxy: true
     enable_return_url: true
     allowed_paths:
       - "/OA_HTML/*"
       - "/webadi/*"
   ```

For complete configuration options, see `config.yaml.template`.  
For WebADI and DBC setup, see [WEBADI_GUIDE.md](WEBADI_GUIDE.md).

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
- `GET /login` - Initiates OIDC authentication flow (supports `?return_url=` parameter)
- `GET /callback` - Handles OIDC provider callback
- `GET /logout` - Terminates EBS session and clears cookies
- `GET /health` - Health check endpoint
- `ANY /OA_HTML/*` - Proxy to EBS (if proxy enabled)
- `ANY /webadi/*` - WebADI proxy (if proxy enabled)
- `ANY /servlets/*` - Servlet proxy (if proxy enabled)

**Proxy Routes:** Additional routes are registered based on `ebs.allowed_paths` configuration.

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
