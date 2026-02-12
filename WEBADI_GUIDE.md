# WebADI and EBS-Dependent Methods Support Guide

## Overview

The EBS SSO Gateway now supports WebADI, Oracle Forms, and other EBS-dependent methods through enhanced session management and request proxying capabilities.

## Features

### 1. Deep Link Support (Return URLs)

The gateway now preserves the original destination URL during authentication, allowing users to:
- Access specific EBS forms directly
- Use WebADI links from Excel
- Bookmark EBS pages
- Navigate directly to reports

**Configuration:**
```yaml
ebs:
  enable_return_url: true
```

**Usage:**
- Direct access: `http://sso-gateway/OA_HTML/some_form` will authenticate and redirect there
- Query parameter: `http://sso-gateway/login?return_url=/OA_HTML/some_form`

### 2. EBS Request Proxy

The gateway can proxy authenticated requests to EBS, maintaining session state.

**Configuration:**
```yaml
ebs:
  enable_proxy: true
  allowed_paths:
    - "/OA_HTML/*"
    - "/webadi/*"
    - "/servlets/*"
    - "/forms/*"
```

**How it works:**
1. User authenticates via OIDC
2. Session is created in EBS
3. Subsequent requests to EBS paths are proxied with session cookies
4. Session validation happens on each request

### 3. WebADI Support

WebADI (Web Application Desktop Integrator) allows Excel-based data integration with EBS.

**Requirements:**
- Enable proxy mode
- Enable return URL support
- Configure allowed paths to include `/webadi/*`
- Set appropriate max_upload_size

**Configuration:**
```yaml
ebs:
  enable_proxy: true
  enable_return_url: true
  max_upload_size: 104857600  # 100MB for Excel uploads
  allowed_paths:
    - "/OA_HTML/*"
    - "/webadi/*"
```

**WebADI Workflow:**
1. User clicks WebADI link in EBS or Excel
2. Request goes to SSO gateway
3. If not authenticated, user logs in via OIDC
4. Gateway creates EBS session
5. User is redirected back to WebADI
6. Excel downloads/uploads work through proxy

### 4. DBC File Support

Instead of direct database credentials, use EBS DBC files for secure connection.

**Benefits:**
- No direct APPS schema password needed
- Uses GUEST_USER_PWD mechanism
- Leverages EBS security framework
- Better integration with EBS architecture

**Configuration:**
```yaml
database:
  use_dbc: true
  dbc_file: "/path/to/secure/appsweb.dbc"
  apps_user: "SYSADMIN"
```

**DBC File Location:**
EBS DBC files are typically located at:
- `$INST_TOP/appl/fnd/12.0.0/secure/appsweb.dbc`
- `$FND_SECURE/appsweb.dbc`

### 5. Trusted Node Configuration

Register the SSO gateway as a trusted node in EBS for enhanced security.

**Steps to Configure:**

1. **Register Node in EBS:**
```sql
-- Login to EBS database as APPS
BEGIN
  FND_NODES_PKG.REGISTER_NODE(
    node_name => 'SSO_GATEWAY_NODE',
    host => 'sso-gateway.yourdomain.com',
    domain => 'yourdomain.com',
    webhost => 'sso-gateway.yourdomain.com',
    support_forms => 'N',
    support_web => 'Y',
    support_admin => 'N',
    support_cp => 'N'
  );
  COMMIT;
END;
/
```

2. **Configure in Gateway:**
```yaml
ebs:
  trusted_node_name: "SSO_GATEWAY_NODE"
  trusted_node_secret: "your-shared-secret"
```

3. **Verify Node Registration:**
```sql
SELECT node_name, server_address, status
FROM fnd_nodes
WHERE node_name = 'SSO_GATEWAY_NODE';
```

### 6. APPS_CONNECT UMX Role

Use application-level authentication instead of direct database access.

**Benefits:**
- Uses EBS security APIs
- Better audit trail
- No need for APPS schema password
- Follows EBS security best practices

**Setup:**

1. **Grant APPS_CONNECT UMX Role:**
```sql
-- In EBS, navigate to: User Management > Users
-- Or use SQL:
BEGIN
  FND_USER_PKG.ADDRESP(
    username => 'SYSADMIN',
    resp_app => 'UMX',
    resp_key => 'APPS_CONNECT',
    security_group => 'STANDARD',
    description => 'SSO Gateway Access'
  );
  COMMIT;
END;
/
```

2. **Configure Gateway:**
```yaml
database:
  use_dbc: true
  apps_user: "SYSADMIN"
```

## Complete Configuration Example

### Production Configuration with All Features

```yaml
environment: "PROD"

server:
  port: 8443
  host: "0.0.0.0"
  base_url: "https://sso.yourdomain.com"

oidc:
  provider_url: "https://login.microsoftonline.com/{tenant}/v2.0"
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  redirect_url: "https://sso.yourdomain.com/callback"
  scopes:
    - "openid"
    - "profile"
    - "email"
  user_claim: "email"

database:
  # DBC file method (recommended)
  use_dbc: true
  dbc_file: "/u01/install/APPS/fs1/inst/apps/PROD_hostname/appl/fnd/12.0.0/secure/appsweb.dbc"
  apps_user: "SYSADMIN"
  
  # Connection pool
  pool_size: 20
  max_open_conns: 50
  max_idle_conns: 10

ebs:
  base_url: "https://ebs.yourdomain.com"
  home_page: "/OA_HTML/AppsLogin"
  cookie_domain: ".yourdomain.com"
  cookie_secure: true
  
  # Trusted node
  trusted_node_name: "SSO_GATEWAY_NODE"
  trusted_node_secret: "SecureNodeSecret123"
  
  # Proxy and WebADI support
  enable_proxy: true
  enable_return_url: true
  max_upload_size: 104857600
  
  allowed_paths:
    - "/OA_HTML/*"
    - "/webadi/*"
    - "/servlets/*"
    - "/forms/*"
    - "/oa_servlets/*"
    - "/servlet/*"

logging:
  level: "info"
  format: "json"
  output: "/var/log/ebssso/ebssso.log"

session:
  timeout_minutes: 480
  cookie_name: "EBS_SSO_SESSION"
```

## Testing WebADI

### Test Procedure:

1. **Access WebADI Upload Template:**
   - Navigate to EBS form with WebADI integration
   - Click "Create Document" or "Download" button
   - Excel should download with data

2. **Upload Data via WebADI:**
   - Make changes in Excel
   - Click "Upload" button in Excel
   - Data should be uploaded to EBS

3. **Verify Session Persistence:**
   - Check that session remains active during WebADI operations
   - Monitor logs for session validation

### Common WebADI URLs:
- Upload: `/webadi/WEB-INF/integrators/upload`
- Download: `/webadi/WEB-INF/integrators/download`
- Integrator: `/webadi/integrators/*`

## Troubleshooting

### WebADI Upload Fails

**Symptoms:** Excel upload fails with authentication error

**Solutions:**
1. Verify session is valid:
   ```bash
   curl -b cookies.txt http://sso-gateway/health
   ```

2. Check upload size limit:
   ```yaml
   ebs:
     max_upload_size: 104857600  # Increase if needed
   ```

3. Verify allowed paths include `/webadi/*`

### Deep Links Not Working

**Symptoms:** Return URL not preserved after login

**Solutions:**
1. Enable return URL support:
   ```yaml
   ebs:
     enable_return_url: true
   ```

2. Check logs for return URL capture:
   ```bash
   grep "return_url" /var/log/ebssso/ebssso.log
   ```

### DBC File Connection Fails

**Symptoms:** Cannot connect using DBC file

**Solutions:**
1. Verify DBC file exists and is readable
2. Check GUEST_USER_PWD encryption in DBC file
3. Ensure proper permissions on DBC file:
   ```bash
   chmod 640 /path/to/appsweb.dbc
   chown ebssso:ebssso /path/to/appsweb.dbc
   ```

### Session Validation Errors

**Symptoms:** Session appears created but validation fails

**Solutions:**
1. Check FND API access:
   ```sql
   SELECT * FROM icx_sessions WHERE session_id = <your_session_id>;
   ```

2. Verify APPS_CONNECT UMX role:
   ```sql
   SELECT responsibility_key 
   FROM fnd_user_resp_groups_direct furg
   JOIN fnd_responsibility_vl fr ON furg.responsibility_id = fr.responsibility_id
   WHERE user_id = (SELECT user_id FROM fnd_user WHERE user_name = 'SYSADMIN')
   AND fr.responsibility_key = 'APPS_CONNECT';
   ```

3. Check trusted node status:
   ```sql
   SELECT * FROM fnd_nodes WHERE node_name = 'SSO_GATEWAY_NODE';
   ```

## Security Considerations

### Path Whitelisting

Always configure `allowed_paths` to limit proxy access:

```yaml
ebs:
  allowed_paths:
    - "/OA_HTML/*"      # Self-service pages
    - "/webadi/*"       # WebADI only
    - "/servlets/oracle.apps.fnd.*"  # Specific servlets
```

### Session Security

- Use HTTPS in production (`cookie_secure: true`)
- Set appropriate cookie domain
- Monitor session validation failures
- Implement session timeout policies

### DBC File Security

- Store DBC files with restrictive permissions (640)
- Keep DBC files outside web root
- Use separate DBC files per environment
- Rotate GUEST_USER_PWD periodically

## Performance Optimization

### For High WebADI Usage:

```yaml
database:
  max_open_conns: 100
  max_idle_conns: 20

ebs:
  max_upload_size: 209715200  # 200MB for large Excel files
```

### Connection Pooling:

```yaml
database:
  pool_size: 50
  max_open_conns: 100
  max_idle_conns: 25
```

## Migration from Direct Connection to DBC

### Step-by-Step Migration:

1. **Backup Current Configuration:**
   ```bash
   cp config.yaml config.yaml.backup
   ```

2. **Locate DBC File:**
   ```bash
   find $INST_TOP -name "appsweb.dbc"
   ```

3. **Copy DBC File:**
   ```bash
   cp $FND_SECURE/appsweb.dbc /opt/ebssso/secure/
   chmod 640 /opt/ebssso/secure/appsweb.dbc
   ```

4. **Update Configuration:**
   ```yaml
   database:
     use_dbc: true
     dbc_file: "/opt/ebssso/secure/appsweb.dbc"
     apps_user: "SYSADMIN"
   ```

5. **Test Connection:**
   ```bash
   ./ebssso &
   curl http://localhost:8080/health
   ```

6. **Verify in Logs:**
   ```bash
   tail -f /var/log/ebssso/ebssso.log | grep "DBC"
   ```

## Support Matrix

| Feature | EBS 12.1 | EBS 12.2 | R12.2+ |
|---------|----------|----------|--------|
| WebADI | ✓ | ✓ | ✓ |
| Forms | ✓ | ✓ | ✓ |
| DBC Files | ✓ | ✓ | ✓ |
| Trusted Nodes | ✗ | ✓ | ✓ |
| APPS_CONNECT UMX | ✗ | ✓ | ✓ |
| Deep Links | ✓ | ✓ | ✓ |

## Additional Resources

- [Oracle E-Business Suite Security Guide](https://docs.oracle.com/cd/E18727_01/index.htm)
- [WebADI Developer's Guide](https://docs.oracle.com/cd/E18727_01/ada.htm)
- [EBS SSO Integration Guide](https://docs.oracle.com/cd/E18727_01/integrate.htm)
