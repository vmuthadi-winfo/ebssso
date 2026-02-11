🚀 The GitHub Copilot "Master Prompt"
Role: You are a Senior Backend Engineer and Oracle Integration Expert. Project: Build a lightweight, independent "EBS SSO Gateway" in Go (Golang) to enable OIDC-based SSO for Oracle E-Business Suite 12.2.

Architectural Requirements:

Framework: Use the Gin web framework for high-performance HTTP handling (10–200 concurrent requests).

OIDC Integration: Implement the OIDC Authorization Code Flow using coreos/go-oidc. It must support dynamic provider configuration for Entra ID, Okta, and Ping.

Independence: The app must run as a standalone Linux binary. Do not use WebLogic or OAM.

Configuration: Use a config.yaml file to define environment-specific settings (Client IDs, Client Secrets, Oracle DB connection strings, and EBS Base URLs).

Detailed Logical Flow:

Step 1: The Challenge: When a user accesses /login, redirect them to the configured OIDC authorization_endpoint.

Step 2: The Callback: On the /callback route, exchange the authorization code for an ID Token. Extract the email or upn claim.

Step 3: Identity Mapping: Look up the USER_NAME in the EBS FND_USER table that matches the OIDC claim.

Step 4: EBS Session Creation (The Core):

Connect to the Oracle Database using godror.

Execute a PL/SQL block that mimics the FND_SESSION_MANAGEMENT logic.

You must call fnd_pub_auth.check_user or a custom wrapper that returns a valid ICX_SESSION_ID.

Step 5: Cookie Injection: Once the session is created, the app must set the following cookies in the user's browser: ebs_session, ICX_SESSION, and any domain-specific cookies required by EBS.

Step 6: Redirect: Finally, redirect the user to the EBS Home Page (e.g., /OA_HTML/AppsLogin).

Technical Constraints & Security:

Concurrency: Use Go's goroutines and a connection pool for Oracle DB to ensure stability under load.

Environment Isolation: Ensure the code reads from an environment variable EBS_ENV (e.g., 'DEV', 'PROD') to load the corresponding YAML configuration.

Logging: Implement structured logging (e.g., zap or logrus) to track login attempts, failures, and session IDs.

Error Handling: Gracefully handle "User Not Found in EBS" or "OIDC Token Expired" scenarios with user-friendly error pages.

Deliverables Requested:

A standard Go project structure (main.go, config/, handlers/, db/).

The config.yaml template.

The OIDC Auth and Callback handler logic.

The Database logic to call the Oracle PL/SQL session creation routine.
