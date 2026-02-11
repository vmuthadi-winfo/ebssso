package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/godror/godror"
	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
)

// OracleDB represents the Oracle database connection
type OracleDB struct {
	db     *sql.DB
	config *config.DatabaseConfig
	logger *logrus.Logger
}

// NewOracleDB creates a new Oracle database connection
func NewOracleDB(cfg *config.DatabaseConfig, logger *logrus.Logger) (*OracleDB, error) {
	// Build connection string
	connStr := fmt.Sprintf("%s/%s@%s",
		cfg.Username,
		cfg.Password,
		cfg.ConnectionString,
	)

	db, err := sql.Open("godror", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Successfully connected to Oracle database")

	return &OracleDB{
		db:     db,
		config: cfg,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (o *OracleDB) Close() error {
	if o.db != nil {
		return o.db.Close()
	}
	return nil
}

// GetUserByEmail looks up a user in FND_USER by email
func (o *OracleDB) GetUserByEmail(ctx context.Context, email string) (string, error) {
	var userName string
	query := `SELECT USER_NAME FROM FND_USER WHERE UPPER(EMAIL_ADDRESS) = UPPER(:email) AND END_DATE IS NULL`

	err := o.db.QueryRowContext(ctx, query, sql.Named("email", email)).Scan(&userName)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found with email: %s", email)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query user: %w", err)
	}

	o.logger.WithFields(logrus.Fields{
		"email":     email,
		"user_name": userName,
	}).Info("Found EBS user")

	return userName, nil
}

// CreateEBSSession creates an EBS session for the given user
func (o *OracleDB) CreateEBSSession(ctx context.Context, userName string) (string, error) {
	var sessionID string

	// PL/SQL block to create EBS session
	// This calls the FND session management APIs to create a valid ICX session
	plsql := `
	DECLARE
		l_session_id NUMBER;
		l_user_id NUMBER;
		l_resp_id NUMBER := NULL;
		l_resp_appl_id NUMBER := NULL;
		l_security_group_id NUMBER := 0;
		l_site_id NUMBER := NULL;
	BEGIN
		-- Get user ID
		SELECT user_id INTO l_user_id
		FROM fnd_user
		WHERE user_name = :user_name
		AND end_date IS NULL;
		
		-- Create ICX session
		-- This mimics what AppsLogin does
		l_session_id := icx_sec.createSession(
			p_user_id => l_user_id,
			p_responsibility_id => l_resp_id,
			p_resp_appl_id => l_resp_appl_id,
			p_security_group_id => l_security_group_id,
			p_site_id => l_site_id
		);
		
		-- Return the session ID
		:session_id := TO_CHAR(l_session_id);
		
	EXCEPTION
		WHEN OTHERS THEN
			RAISE_APPLICATION_ERROR(-20001, 'Failed to create EBS session: ' || SQLERRM);
	END;
	`

	// Execute PL/SQL block
	_, err := o.db.ExecContext(ctx, plsql,
		sql.Named("user_name", userName),
		sql.Named("session_id", sql.Out{Dest: &sessionID}),
	)

	if err != nil {
		o.logger.WithFields(logrus.Fields{
			"user_name": userName,
			"error":     err.Error(),
		}).Error("Failed to create EBS session")
		return "", fmt.Errorf("failed to create EBS session: %w", err)
	}

	o.logger.WithFields(logrus.Fields{
		"user_name":  userName,
		"session_id": sessionID,
	}).Info("Created EBS session")

	return sessionID, nil
}

// ValidateSession checks if a session ID is valid
func (o *OracleDB) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM icx_sessions
		WHERE session_id = :session_id
		AND disabled_flag != 'Y'
		AND (last_connect + limit_time / 86400) > SYSDATE
	`

	err := o.db.QueryRowContext(ctx, query, sql.Named("session_id", sessionID)).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to validate session: %w", err)
	}

	return count > 0, nil
}

// EndSession terminates an EBS session
func (o *OracleDB) EndSession(ctx context.Context, sessionID string) error {
	query := `
		UPDATE icx_sessions
		SET disabled_flag = 'Y',
		    last_connect = SYSDATE
		WHERE session_id = :session_id
	`

	_, err := o.db.ExecContext(ctx, query, sql.Named("session_id", sessionID))
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	o.logger.WithField("session_id", sessionID).Info("Ended EBS session")
	return nil
}
