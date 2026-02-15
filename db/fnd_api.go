package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
)

// FNDAPIClient handles EBS FND API calls using APPS_CONNECT UMX role
type FNDAPIClient struct {
	db     *sql.DB
	logger *logrus.Logger
}

// NewFNDAPIClient creates a new FND API client
func NewFNDAPIClient(db *sql.DB, logger *logrus.Logger) *FNDAPIClient {
	return &FNDAPIClient{
		db:     db,
		logger: logger,
	}
}

// GetUserIDByEmail looks up user ID in EBS using FND_USER_PKG
func (f *FNDAPIClient) GetUserIDByEmail(ctx context.Context, email string) (int64, string, error) {
	var userID int64
	var userName string
	
	// Use FND API to query user information
	query := `
		SELECT user_id, user_name
		FROM fnd_user
		WHERE UPPER(email_address) = UPPER(:email)
		AND end_date IS NULL
		AND ROWNUM = 1
	`
	
	err := f.db.QueryRowContext(ctx, query, 
		sql.Named("email", email),
	).Scan(&userID, &userName)
	
	if err == sql.ErrNoRows {
		return 0, "", fmt.Errorf("user not found with email: %s", email)
	}
	if err != nil {
		return 0, "", fmt.Errorf("failed to query user: %w", err)
	}
	
	f.logger.WithFields(logrus.Fields{
		"email":     email,
		"user_id":   userID,
		"user_name": userName,
	}).Info("Found EBS user via FND API")
	
	return userID, userName, nil
}

// InitializeSession initializes FND_GLOBAL context for the session
func (f *FNDAPIClient) InitializeSession(ctx context.Context, userID int64, respID, respAppID, securityGroupID int64) error {
	plsql := `
	BEGIN
		-- Initialize FND_GLOBAL context
		FND_GLOBAL.APPS_INITIALIZE(
			user_id => :user_id,
			resp_id => :resp_id,
			resp_appl_id => :resp_appl_id,
			security_group_id => :security_group_id
		);
	END;
	`
	
	_, err := f.db.ExecContext(ctx, plsql,
		sql.Named("user_id", userID),
		sql.Named("resp_id", respID),
		sql.Named("resp_appl_id", respAppID),
		sql.Named("security_group_id", securityGroupID),
	)
	
	if err != nil {
		return fmt.Errorf("failed to initialize FND_GLOBAL: %w", err)
	}
	
	f.logger.WithFields(logrus.Fields{
		"user_id":           userID,
		"resp_id":           respID,
		"resp_appl_id":      respAppID,
		"security_group_id": securityGroupID,
	}).Debug("Initialized FND_GLOBAL context")
	
	return nil
}

// CreateSessionViaSSOAuth creates an EBS session using SSO authentication approach
// This method uses the trusted node approach with APPS_CONNECT UMX role
func (f *FNDAPIClient) CreateSessionViaSSOAuth(ctx context.Context, userID int64, userName string) (string, error) {
	var sessionID string
	
	// Use ICX_SEC package with SSO authentication flag
	plsql := `
	DECLARE
		l_session_id NUMBER;
		l_return_status VARCHAR2(1);
		l_msg_count NUMBER;
		l_msg_data VARCHAR2(2000);
	BEGIN
		-- Create session using ICX_SEC for SSO
		-- This approach doesn't require direct APPS password
		l_session_id := ICX_SEC.CREATESESSION(
			p_user_id => :user_id,
			p_responsibility_id => NULL,
			p_resp_appl_id => NULL,
			p_security_group_id => 0,
			p_site_id => NULL
		);
		
		IF l_session_id IS NOT NULL AND l_session_id > 0 THEN
			:session_id := TO_CHAR(l_session_id);
			:return_status := 'S';
		ELSE
			:return_status := 'E';
			:session_id := NULL;
		END IF;
		
	EXCEPTION
		WHEN OTHERS THEN
			:return_status := 'E';
			:session_id := NULL;
			RAISE_APPLICATION_ERROR(-20001, 'Failed to create SSO session: ' || SQLERRM);
	END;
	`
	
	var returnStatus string
	
	_, err := f.db.ExecContext(ctx, plsql,
		sql.Named("user_id", userID),
		sql.Named("session_id", sql.Out{Dest: &sessionID}),
		sql.Named("return_status", sql.Out{Dest: &returnStatus}),
	)
	
	if err != nil {
		f.logger.WithFields(logrus.Fields{
			"user_id":   userID,
			"user_name": userName,
			"error":     err.Error(),
		}).Error("Failed to create EBS session via SSO auth")
		return "", fmt.Errorf("failed to create SSO session: %w", err)
	}
	
	if returnStatus != "S" || sessionID == "" {
		return "", fmt.Errorf("session creation failed for user %s", userName)
	}
	
	f.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"user_name":  userName,
		"session_id": sessionID,
	}).Info("Created EBS session via SSO auth")
	
	return sessionID, nil
}

// ValidateAndExtendSession validates an existing session and extends it
func (f *FNDAPIClient) ValidateAndExtendSession(ctx context.Context, sessionID string) (bool, error) {
	var isValid string
	
	plsql := `
	DECLARE
		l_valid BOOLEAN;
	BEGIN
		-- Validate session using ICX_SEC
		l_valid := ICX_SEC.VALIDATESESSION(
			p_session_id => TO_NUMBER(:session_id)
		);
		
		IF l_valid THEN
			:is_valid := 'Y';
			-- Update last_connect to extend session
			UPDATE icx_sessions
			SET last_connect = SYSDATE
			WHERE session_id = TO_NUMBER(:session_id);
		ELSE
			:is_valid := 'N';
		END IF;
	END;
	`
	
	_, err := f.db.ExecContext(ctx, plsql,
		sql.Named("session_id", sessionID),
		sql.Named("is_valid", sql.Out{Dest: &isValid}),
	)
	
	if err != nil {
		return false, fmt.Errorf("failed to validate session: %w", err)
	}
	
	return isValid == "Y", nil
}

// GetUserResponsibilities retrieves available responsibilities for a user
func (f *FNDAPIClient) GetUserResponsibilities(ctx context.Context, userID int64) ([]Responsibility, error) {
	query := `
		SELECT DISTINCT
			fr.responsibility_id,
			fr.responsibility_key,
			fr.responsibility_name,
			fr.application_id
		FROM fnd_user_resp_groups furg
		JOIN fnd_responsibility_vl fr 
			ON furg.responsibility_id = fr.responsibility_id
			AND furg.responsibility_application_id = fr.application_id
		WHERE furg.user_id = :user_id
		AND furg.end_date IS NULL
		AND fr.end_date IS NULL
		ORDER BY fr.responsibility_name
	`
	
	rows, err := f.db.QueryContext(ctx, query, sql.Named("user_id", userID))
	if err != nil {
		return nil, fmt.Errorf("failed to query responsibilities: %w", err)
	}
	defer rows.Close()
	
	var responsibilities []Responsibility
	for rows.Next() {
		var resp Responsibility
		if err := rows.Scan(&resp.ID, &resp.Key, &resp.Name, &resp.ApplicationID); err != nil {
			return nil, fmt.Errorf("failed to scan responsibility: %w", err)
		}
		responsibilities = append(responsibilities, resp)
	}
	
	return responsibilities, nil
}

// Responsibility represents an EBS responsibility
type Responsibility struct {
	ID            int64
	Key           string
	Name          string
	ApplicationID int64
}

// TerminateSession terminates an EBS session using FND APIs
func (f *FNDAPIClient) TerminateSession(ctx context.Context, sessionID string) error {
	plsql := `
	BEGIN
		-- Terminate session using ICX_SEC
		ICX_SEC.ENDSESSION(
			p_session_id => TO_NUMBER(:session_id)
		);
	EXCEPTION
		WHEN OTHERS THEN
			-- Log but don't fail - session might already be ended
			NULL;
	END;
	`
	
	_, err := f.db.ExecContext(ctx, plsql, sql.Named("session_id", sessionID))
	if err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}
	
	f.logger.WithField("session_id", sessionID).Info("Terminated EBS session via FND API")
	return nil
}
