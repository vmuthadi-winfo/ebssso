# Dependency Analysis and Failure Scenarios

## Overview
This document provides a comprehensive analysis of dependencies, potential failure scenarios, and resilience features implemented in the EBS SSO Gateway.

## Critical Dependencies

### 1. OIDC Provider (Entra ID / Okta / Ping)
**Dependency Type:** External Authentication Service

**Required For:**
- User authentication
- Token issuance and verification
- Group information retrieval

**Failure Scenarios:**
- **Provider Unavailable**: Network issues, service outage
  - **Impact**: Users cannot authenticate
  - **Mitigation**: Circuit breaker pattern, retry logic, timeout configuration
  
- **Token Validation Failure**: Invalid token, expired certificate
  - **Impact**: Authentication fails
  - **Mitigation**: Token caching, certificate validation with proper error handling
  
- **Slow Response**: High latency, timeout
  - **Impact**: User experience degradation
  - **Mitigation**: Configurable timeouts (default: 30s), retry with backoff

**Configuration:**
```yaml
oidc:
  timeout: 30
  max_retries: 3
  retry_delay: 2
```

**Health Check:**
- OIDC provider connectivity tested during initialization
- Circuit breaker monitors provider health
- Failed requests trigger circuit breaker after threshold

### 2. Oracle Database (EBS)
**Dependency Type:** Data Persistence and Session Management

**Required For:**
- User lookup in FND_USER
- EBS session creation
- Session validation
- Session termination

**Failure Scenarios:**
- **Connection Failure**: Network issues, database down, invalid credentials
  - **Impact**: Cannot create/validate sessions
  - **Mitigation**: Connection pooling, retry logic, circuit breaker
  
- **Query Timeout**: Long-running queries, database lock
  - **Impact**: Slow response, potential timeout
  - **Mitigation**: Query timeout configuration, connection timeout
  
- **User Not Found**: User doesn't exist in FND_USER
  - **Impact**: Authentication fails with user-friendly error
  - **Mitigation**: Clear error message, audit logging
  
- **Session Creation Failure**: PL/SQL errors, insufficient permissions
  - **Impact**: Cannot create EBS session
  - **Mitigation**: Retry logic, error logging, fallback mechanisms

**Configuration:**
```yaml
database:
  pool_size: 20
  max_open_conns: 50
  max_idle_conns: 10

resilience:
  database_timeout: 10
  enable_retry: true
  max_retries: 3
```

**Health Check:**
- Database connectivity tested via health endpoint
- Connection pool health monitored
- Failed operations tracked by circuit breaker

### 3. EBS Application Server
**Dependency Type:** Target Application

**Required For:**
- Final authentication destination
- Form and servlet access
- WebADI operations

**Failure Scenarios:**
- **Server Unavailable**: Application down, network issues
  - **Impact**: Redirect fails, users see error
  - **Mitigation**: Circuit breaker, health checks, retry for proxy requests
  
- **Session Not Recognized**: EBS doesn't recognize session ID
  - **Impact**: User prompted to login again
  - **Mitigation**: Session validation before redirect, proper cookie setting
  
- **Timeout**: Slow EBS response
  - **Impact**: User experience degradation
  - **Mitigation**: Configurable proxy timeout

**Configuration:**
```yaml
resilience:
  ebs_timeout: 30

ebs:
  enable_proxy: true
```

### 4. Network Infrastructure
**Dependency Type:** Connectivity

**Required For:**
- All inter-service communication
- DNS resolution
- SSL/TLS connections

**Failure Scenarios:**
- **DNS Resolution Failure**: Cannot resolve hostnames
  - **Impact**: Cannot connect to dependencies
  - **Mitigation**: Use IP addresses in critical paths, multiple DNS servers
  
- **Firewall/Port Blocking**: Required ports blocked
  - **Impact**: Connection refused errors
  - **Mitigation**: Document required ports, health checks
  
- **SSL/TLS Certificate Issues**: Expired, invalid, or self-signed certificates
  - **Impact**: Connection fails with SSL error
  - **Mitigation**: Certificate validation configuration, proper cert management

**Required Ports:**
- 443: OIDC provider (HTTPS)
- 1521: Oracle database
- 8000: EBS application server (configurable)
- 8080: SSO Gateway (configurable)

### 5. DBC File (if using DBC method)
**Dependency Type:** Configuration File

**Required For:**
- Database connection parameters
- GUEST_USER_PWD authentication

**Failure Scenarios:**
- **File Not Found**: DBC file missing or inaccessible
  - **Impact**: Cannot start application
  - **Mitigation**: Validation at startup, clear error message
  
- **Decryption Failure**: Cannot decrypt GUEST_USER_PWD
  - **Impact**: Cannot connect to database
  - **Mitigation**: Validation at startup, fallback to legacy method
  
- **Invalid Format**: Corrupted or invalid DBC file
  - **Impact**: Parsing errors
  - **Mitigation**: Format validation, error handling

**Configuration:**
```yaml
database:
  use_dbc: true
  dbc_file: "/path/to/secure/appsweb.dbc"
```

## Session Timeout Management

### EBS Session Timeout Issues

**Problem:** EBS sessions have their own timeout configuration (typically 8 hours), which may not align with SSO gateway session management.

**Impact:**
1. **Mismatch**: SSO session valid but EBS session expired
   - User sees EBS login screen despite having valid SSO session
   
2. **No Warning**: User loses work when session expires unexpectedly
   
3. **No Refresh**: Sessions expire even during active use

**Solutions Implemented:**

### 1. Session Synchronization
```yaml
session:
  sync_with_ebs: true
  timeout_minutes: 480  # Should match EBS profile timeout
```

- Configure SSO timeout to match EBS profile `ICX_SESSION_TIMEOUT`
- Validate EBS session before redirects
- End EBS session when SSO session ends

### 2. Automatic Session Refresh
```yaml
session:
  enable_auto_refresh: true
  refresh_before_expiry: 5
```

- Automatically extend session on activity
- Refresh triggered X minutes before expiry
- Updates both SSO and EBS session timestamps

### 3. Session Activity Tracking
```yaml
session:
  enable_session_tracking: true
  cleanup_interval: 10
```

- Track all active sessions
- Monitor last activity time
- Automatic cleanup of expired sessions
- Update EBS session on activity

### 4. Expiry Warning
```yaml
session:
  expiry_warning_minutes: 10
  grace_period_minutes: 5
```

- Warn users before session expires
- Provide grace period for session refresh
- Allow users to save work

### 5. Concurrent Session Management
```yaml
session:
  max_concurrent_sessions: 3
```

- Limit concurrent sessions per user
- Automatically terminate oldest session when limit reached
- Prevent session exhaustion

## Resilience Features

### Circuit Breaker Pattern

**Purpose:** Prevent cascading failures by temporarily blocking requests to failing dependencies

**Configuration:**
```yaml
resilience:
  enable_circuit_breaker: true
  failure_threshold: 5        # Open circuit after 5 failures
  success_threshold: 2        # Close after 2 successes
  circuit_timeout: 60         # Try again after 60 seconds
```

**States:**
- **Closed**: Normal operation, requests pass through
- **Open**: Dependency failing, requests blocked immediately
- **Half-Open**: Testing if dependency recovered

**Applied To:**
- OIDC provider connections
- Database operations
- EBS proxy requests

**Benefits:**
- Fast failure instead of waiting for timeouts
- Automatic recovery testing
- Prevents overwhelming failed dependencies

### Retry Logic

**Purpose:** Automatically retry transient failures

**Configuration:**
```yaml
resilience:
  enable_retry: true
  max_retries: 3
  retry_delay: 2
  retry_backoff: true
```

**Retry Strategy:**
1. Immediate first attempt
2. Wait 2 seconds, retry (attempt 2)
3. Wait 4 seconds, retry (attempt 3)
4. Wait 6 seconds, retry (attempt 4)
5. Fail with error

**Applied To:**
- OIDC token exchange
- Database queries
- EBS session creation

**Retryable Errors:**
- Network timeouts
- Connection refused
- Temporary database errors
- DNS resolution failures

**Non-Retryable Errors:**
- Authentication failures (401)
- Authorization failures (403)
- User not found (404)
- Invalid requests (400)

### Timeout Configuration

**Purpose:** Prevent indefinite waiting for responses

**Configuration:**
```yaml
resilience:
  database_timeout: 10
  oidc_timeout: 30
  ebs_timeout: 30
```

**Applied To:**
- All database operations
- OIDC provider communication
- EBS proxy requests

**Benefits:**
- Predictable response times
- Resource cleanup on timeout
- Better error handling

### Connection Pooling

**Purpose:** Efficient database connection management

**Configuration:**
```yaml
database:
  pool_size: 20
  max_open_conns: 50
  max_idle_conns: 10
```

**Features:**
- Connection reuse
- Maximum connection limit
- Idle connection management
- Connection health checks

## Group-Based Authorization

### AD Group Filtering

**Purpose:** Control access based on Active Directory group membership

**Configuration:**
```yaml
oidc:
  group_claim: "groups"
  allowed_groups:
    - "EBS_Users"
    - "Finance_Team"
  denied_groups:
    - "Contractors"
  require_group_match: true
  group_match_strategy: "any"

authorization:
  enable_authorization: true
  mode: "strict"
  on_failure_action: "deny"
```

### Authorization Strategies

**1. Permissive Mode (Default)**
- Allow access by default
- Only deny if in denied group
- Good for gradual rollout

**2. Strict Mode**
- Require explicit group membership
- Deny by default
- Better security for production

### Group Matching

**Any Strategy** (default):
- User must be in at least ONE allowed group
- Use for role-based access

**All Strategy**:
- User must be in ALL allowed groups
- Use for strict security requirements

### Wildcard Support
```yaml
allowed_groups:
  - "EBS_*"           # All groups starting with EBS_
  - "*_Admins"        # All admin groups
  - "*Finance*"       # All groups containing Finance
```

### Audit Logging
```yaml
authorization:
  enable_audit_log: true
  audit_log_path: "/var/log/ebssso/audit.log"
```

**Logged Information:**
- User email
- Groups extracted from token
- Authorization decision (ALLOWED/DENIED)
- Reason for decision
- Timestamp

## Monitoring and Health Checks

### Health Endpoint Enhancements

**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "healthy",
  "database": true,
  "circuit_breakers": {
    "oidc": "closed",
    "database": "closed",
    "ebs": "closed"
  },
  "sessions": {
    "tracking_enabled": true,
    "total_sessions": 45,
    "total_users": 42,
    "sessions_near_expiry": 3
  },
  "timestamp": "2026-02-12T05:22:10Z"
}
```

### Metrics Available

1. **Circuit Breaker States**
   - Current state of each circuit
   - Failure counts
   - Last state change

2. **Session Statistics**
   - Active session count
   - Users with active sessions
   - Sessions near expiry
   - Concurrent sessions per user

3. **Database Health**
   - Connection pool status
   - Query performance
   - Failed operations

## Failure Response Strategies

### User-Facing Errors

**Principle:** Provide helpful information without exposing security details

**Error Types:**

1. **OIDC Provider Unavailable**
   ```
   "Authentication service temporarily unavailable. Please try again in a few minutes."
   ```

2. **Database Connection Failed**
   ```
   "Unable to connect to authentication system. Please contact your administrator."
   ```

3. **User Not Found**
   ```
   "User 'user@example.com' not found in EBS. Please contact your administrator."
   ```

4. **Authorization Failed**
   ```
   "Access denied: User not in required groups"
   ```

5. **Session Creation Failed**
   ```
   "Failed to create session. Please try again or contact your administrator."
   ```

### Logging Strategy

**Principle:** Log everything for troubleshooting, but sanitize sensitive data

**Log Levels:**
- **DEBUG**: Detailed flow, claims, group matching
- **INFO**: Successful operations, session creation
- **WARN**: Retries, circuit breaker state changes, authorization failures
- **ERROR**: All failures with context

**Sensitive Data Handling:**
- Never log passwords or secrets
- Sanitize tokens (show first/last 4 characters)
- Log user emails for audit
- Log groups for troubleshooting

## Best Practices

### Production Deployment

1. **Enable All Resilience Features**
   ```yaml
   resilience:
     enable_circuit_breaker: true
     enable_retry: true
     enable_health_check: true
   ```

2. **Configure Session Management**
   ```yaml
   session:
     enable_session_tracking: true
     enable_auto_refresh: true
     max_concurrent_sessions: 3
   ```

3. **Enable Authorization**
   ```yaml
   authorization:
     enable_authorization: true
     mode: "strict"
     enable_audit_log: true
   ```

4. **Set Appropriate Timeouts**
   ```yaml
   resilience:
     database_timeout: 10
     oidc_timeout: 30
     ebs_timeout: 30
   ```

5. **Monitor Health**
   - Check `/health` endpoint regularly
   - Alert on circuit breaker opens
   - Monitor session counts
   - Track authorization denials

### Security Considerations

1. **Group-Based Access**
   - Start with permissive mode
   - Monitor audit logs
   - Gradually move to strict mode
   - Regularly review group memberships

2. **Session Security**
   - Use HTTPS in production
   - Set secure cookie flags
   - Limit concurrent sessions
   - Enable session tracking

3. **Error Handling**
   - Don't expose internal errors
   - Log detailed errors server-side
   - Provide helpful user messages
   - Track failed auth attempts

### Performance Optimization

1. **Connection Pooling**
   - Size based on concurrent users
   - Monitor pool utilization
   - Adjust based on load

2. **Session Caching**
   - Enable session tracking
   - Reduce database queries
   - Improve response time

3. **Circuit Breakers**
   - Prevent timeout delays
   - Fast failure
   - Automatic recovery

## Troubleshooting Guide

### Issue: Circuit Breaker Open

**Symptoms:** Users see "service unavailable" errors

**Diagnosis:**
```bash
curl http://localhost:8080/health
# Check circuit_breakers status
```

**Resolution:**
1. Check dependency availability
2. Review error logs
3. Wait for automatic recovery
4. Restart if needed

### Issue: Sessions Expiring Too Soon

**Symptoms:** Users logged out unexpectedly

**Diagnosis:**
- Check session timeout configuration
- Compare with EBS profile timeout
- Review session tracking logs

**Resolution:**
```yaml
session:
  timeout_minutes: 480  # Match EBS timeout
  enable_auto_refresh: true
  sync_with_ebs: true
```

### Issue: Authorization Failures

**Symptoms:** Users denied access

**Diagnosis:**
- Check audit logs for denial reasons
- Verify group claim in OIDC token
- Review group configuration

**Resolution:**
1. Verify user has correct AD groups
2. Check group matching strategy
3. Review allowed/denied groups configuration
4. Test with permissive mode first

## Summary

This enhanced EBS SSO Gateway provides:

✅ **Comprehensive dependency management**
✅ **Resilient failure handling**
✅ **Session timeout synchronization**
✅ **AD group-based authorization**
✅ **Circuit breaker protection**
✅ **Automatic retry logic**
✅ **Session tracking and management**
✅ **Detailed monitoring and health checks**
✅ **Audit logging for compliance**
✅ **Production-ready configuration**

All features are configurable and can be enabled/disabled based on requirements.
