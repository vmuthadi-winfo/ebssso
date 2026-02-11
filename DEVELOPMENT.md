# EBS SSO Gateway - Development Guide

## Quick Start

### 1. Initial Setup
```bash
# Clone repository
git clone https://github.com/vmuthadi-winfo/ebssso.git
cd ebssso

# Install dependencies
make install-deps

# Create configuration from template
cp config.yaml.template config.yaml
```

### 2. Configure for Development
Edit `config.yaml`:

```yaml
environment: "DEV"

server:
  port: 8080
  host: "0.0.0.0"
  base_url: "http://localhost:8080"

oidc:
  # For Entra ID (Azure AD)
  provider_url: "https://login.microsoftonline.com/{tenant-id}/v2.0"
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  redirect_url: "http://localhost:8080/callback"
  scopes:
    - "openid"
    - "profile"
    - "email"
  user_claim: "email"  # or "upn" for Entra ID

database:
  username: "apps"
  password: "apps_password"
  connection_string: "localhost:1521/EBSDB"
  pool_size: 10
  max_open_conns: 20
  max_idle_conns: 5

ebs:
  base_url: "http://ebs-server:8000"
  home_page: "/OA_HTML/AppsLogin"
  cookie_domain: "localhost"
  cookie_secure: false

logging:
  level: "debug"
  format: "json"
  output: "stdout"
```

### 3. Build and Run
```bash
# Build
make build

# Run
./ebssso
```

## OIDC Provider Configuration

### Microsoft Entra ID (Azure AD)

1. **Register Application**
   - Go to Azure Portal → Azure Active Directory → App registrations
   - Click "New registration"
   - Set Redirect URI: `http://localhost:8080/callback` (dev) or your production URL
   - Note the Application (client) ID

2. **Create Client Secret**
   - Go to Certificates & secrets
   - Create new client secret
   - Copy the secret value immediately

3. **Configure API Permissions**
   - Add permissions: `openid`, `profile`, `email`
   - Grant admin consent if required

4. **Update config.yaml**
   ```yaml
   oidc:
     provider_url: "https://login.microsoftonline.com/{tenant-id}/v2.0"
     client_id: "{application-id}"
     client_secret: "{client-secret}"
     user_claim: "email"  # or "upn"
   ```

### Okta

1. **Create OIDC Application**
   - Go to Applications → Create App Integration
   - Choose "OIDC - OpenID Connect"
   - Choose "Web Application"
   - Set Sign-in redirect URIs: `http://localhost:8080/callback`

2. **Update config.yaml**
   ```yaml
   oidc:
     provider_url: "https://your-domain.okta.com"
     client_id: "{client-id}"
     client_secret: "{client-secret}"
     user_claim: "email"
   ```

### Ping Identity

1. **Create Application**
   - Go to Connections → Applications
   - Create new OIDC Web App
   - Set Redirect URI: `http://localhost:8080/callback`

2. **Update config.yaml**
   ```yaml
   oidc:
     provider_url: "https://auth.pingone.com/{environment-id}/as"
     client_id: "{client-id}"
     client_secret: "{client-secret}"
     user_claim: "email"
   ```

## Oracle Database Setup

### Required Database Objects

The EBS SSO Gateway requires access to:

1. **Tables**
   - `FND_USER` - User lookup
   - `ICX_SESSIONS` - Session management

2. **Packages**
   - `ICX_SEC` - Session creation

### Create Database User (if needed)

```sql
-- Connect as APPS user
CREATE USER ebssso IDENTIFIED BY your_password;
GRANT CONNECT, RESOURCE TO ebssso;

-- Grant necessary permissions
GRANT SELECT ON APPS.FND_USER TO ebssso;
GRANT SELECT, UPDATE ON APPS.ICX_SESSIONS TO ebssso;
GRANT EXECUTE ON APPS.ICX_SEC TO ebssso;
```

### Test Database Connection

```bash
sqlplus apps/apps_password@localhost:1521/EBSDB

-- Test user lookup
SELECT user_name, email_address 
FROM fnd_user 
WHERE email_address = 'test@example.com' 
AND end_date IS NULL;

-- Test session package
DECLARE
  l_session_id NUMBER;
BEGIN
  l_session_id := icx_sec.validateSession(12345);
  DBMS_OUTPUT.PUT_LINE('Session validation works');
END;
/
```

## Development Workflow

### Running with Live Reload

For development with automatic reloading, use `air`:

```bash
# Install air
go install github.com/cosmtrek/air@latest

# Create .air.toml configuration
cat > .air.toml << EOF
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/ebssso ."
  bin = "tmp/ebssso"
  include_ext = ["go", "yaml", "html"]
  exclude_dir = ["tmp", "vendor"]
  delay = 1000

[log]
  time = true
EOF

# Run with air
air
```

### Debugging

#### Using Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug

# In another terminal, connect
dlv connect localhost:2345
```

#### Using VS Code

Add to `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch EBS SSO",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}",
      "env": {
        "EBS_ENV": "DEV"
      }
    }
  ]
}
```

## Testing

### Manual Testing Flow

1. **Start Application**
   ```bash
   ./ebssso
   ```

2. **Test Login Flow**
   - Open browser: `http://localhost:8080/login`
   - Redirected to OIDC provider
   - Complete authentication
   - Verify redirect to callback
   - Check session creation
   - Verify redirect to EBS

3. **Check Logs**
   ```bash
   # View logs
   tail -f logs/ebssso.log

   # Or if using systemd
   journalctl -u ebssso -f
   ```

4. **Test Health Endpoint**
   ```bash
   curl http://localhost:8080/health
   ```

### Testing with Mock OIDC Provider

For testing without a real OIDC provider, you can use a mock:

```bash
# Install oidc-mock
docker run -d \
  -p 4011:4011 \
  -e PORT=4011 \
  -e CLIENT_ID=mock-client-id \
  -e CLIENT_SECRET=mock-client-secret \
  -e CLIENT_REDIRECT_URI=http://localhost:8080/callback \
  ghcr.io/navikt/mock-oauth2-server:0.5.8
```

Update config.yaml:
```yaml
oidc:
  provider_url: "http://localhost:4011/default"
  client_id: "mock-client-id"
  client_secret: "mock-client-secret"
```

## Common Issues

### Issue: Database Connection Failed

**Solution:**
```bash
# Check Oracle listener
lsnrctl status

# Test connection
sqlplus apps/password@connection_string

# Check environment variables
echo $ORACLE_HOME
echo $LD_LIBRARY_PATH
```

### Issue: OIDC Provider Connection Failed

**Solution:**
- Verify provider URL is correct
- Check network connectivity
- Verify client ID and secret
- Check redirect URI matches provider configuration

### Issue: User Not Found in EBS

**Solution:**
```sql
-- Check user in database
SELECT user_name, email_address, end_date
FROM fnd_user
WHERE UPPER(email_address) = UPPER('user@example.com');

-- Ensure user is active (end_date is null)
UPDATE fnd_user
SET end_date = NULL
WHERE user_name = 'USERNAME';
```

### Issue: Session Creation Failed

**Solution:**
- Verify ICX_SEC package is accessible
- Check database user has EXECUTE permission
- Review Oracle database logs
- Ensure EBS instance is properly configured

## Performance Tuning

### Database Connection Pool

Adjust based on load:
```yaml
database:
  pool_size: 20        # Concurrent connections
  max_open_conns: 50   # Maximum connections
  max_idle_conns: 10   # Idle connections to maintain
```

### Gin Performance Mode

For production:
```go
gin.SetMode(gin.ReleaseMode)
```

### Logging Level

In production, use `info` or `warn`:
```yaml
logging:
  level: "info"  # Reduce log verbosity
```

## Monitoring

### Health Check

```bash
# Basic health check
curl http://localhost:8080/health

# With authentication monitoring
watch -n 5 'curl -s http://localhost:8080/health | jq'
```

### Metrics (Future Enhancement)

Consider adding Prometheus metrics:
- Login attempts (success/failure)
- Session creation time
- Database query duration
- Active sessions

## Deployment Checklist

- [ ] Update config.yaml with production settings
- [ ] Set `cookie_secure: true` for HTTPS
- [ ] Configure proper `cookie_domain`
- [ ] Set `logging.level: "info"`
- [ ] Enable firewall rules for port 8080
- [ ] Set up SSL/TLS termination (nginx/apache)
- [ ] Configure systemd service
- [ ] Set up log rotation
- [ ] Test failover scenarios
- [ ] Document production URLs and contacts

## Additional Resources

- [Go OIDC Documentation](https://github.com/coreos/go-oidc)
- [Gin Framework Guide](https://gin-gonic.com/docs/)
- [Oracle E-Business Suite Documentation](https://docs.oracle.com/cd/E18727_01/index.htm)
- [OIDC Specification](https://openid.net/connect/)
