package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
	"github.com/vmuthadi-winfo/ebssso/db"
)

// SessionInfo holds information about an active session
type SessionInfo struct {
	SessionID    string
	UserEmail    string
	UserName     string
	CreatedAt    time.Time
	LastActivity time.Time
	ExpiresAt    time.Time
}

// SessionManager manages session lifecycle and tracking
type SessionManager struct {
	config   *config.Config
	db       *db.OracleDB
	logger   *logrus.Logger
	sessions map[string]*SessionInfo // sessionID -> SessionInfo
	userSessions map[string][]string  // userEmail -> []sessionID
	mutex    sync.RWMutex
	stopChan chan bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(cfg *config.Config, database *db.OracleDB, logger *logrus.Logger) *SessionManager {
	sm := &SessionManager{
		config:       cfg,
		db:           database,
		logger:       logger,
		sessions:     make(map[string]*SessionInfo),
		userSessions: make(map[string][]string),
		stopChan:     make(chan bool),
	}
	
	// Start cleanup goroutine if session tracking is enabled
	if cfg.Session.EnableSessionTracking {
		go sm.cleanupExpiredSessions()
	}
	
	return sm
}

// TrackSession adds a session to tracking
func (sm *SessionManager) TrackSession(sessionID, userEmail, userName string) error {
	if !sm.config.Session.EnableSessionTracking {
		return nil
	}
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	// Check concurrent session limit
	if sm.config.Session.MaxConcurrentSessions > 0 {
		userSessions := sm.userSessions[userEmail]
		if len(userSessions) >= sm.config.Session.MaxConcurrentSessions {
			// Remove oldest session
			oldestSessionID := userSessions[0]
			sm.removeSessionLocked(oldestSessionID)
			sm.logger.WithFields(logrus.Fields{
				"user":       userEmail,
				"session_id": oldestSessionID,
			}).Info("Removed oldest session due to concurrent session limit")
		}
	}
	
	// Add new session
	now := time.Now()
	expiresAt := now.Add(time.Duration(sm.config.Session.TimeoutMinutes) * time.Minute)
	
	sessionInfo := &SessionInfo{
		SessionID:    sessionID,
		UserEmail:    userEmail,
		UserName:     userName,
		CreatedAt:    now,
		LastActivity: now,
		ExpiresAt:    expiresAt,
	}
	
	sm.sessions[sessionID] = sessionInfo
	sm.userSessions[userEmail] = append(sm.userSessions[userEmail], sessionID)
	
	sm.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"user_email": userEmail,
		"user_name":  userName,
		"expires_at": expiresAt,
	}).Info("Session tracked")
	
	return nil
}

// UpdateActivity updates the last activity time for a session
func (sm *SessionManager) UpdateActivity(sessionID string) {
	if !sm.config.Session.EnableSessionTracking {
		return
	}
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	if session, exists := sm.sessions[sessionID]; exists {
		session.LastActivity = time.Now()
		
		// Extend expiry if auto-refresh is enabled
		if sm.config.Session.EnableAutoRefresh {
			session.ExpiresAt = time.Now().Add(time.Duration(sm.config.Session.TimeoutMinutes) * time.Minute)
		}
	}
}

// RemoveSession removes a session from tracking
func (sm *SessionManager) RemoveSession(sessionID string) {
	if !sm.config.Session.EnableSessionTracking {
		return
	}
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	sm.removeSessionLocked(sessionID)
}

// removeSessionLocked removes a session (must be called with mutex locked)
func (sm *SessionManager) removeSessionLocked(sessionID string) {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return
	}
	
	// Remove from sessions map
	delete(sm.sessions, sessionID)
	
	// Remove from user sessions
	userSessions := sm.userSessions[session.UserEmail]
	for i, sid := range userSessions {
		if sid == sessionID {
			sm.userSessions[session.UserEmail] = append(userSessions[:i], userSessions[i+1:]...)
			break
		}
	}
	
	// Clean up empty user session lists
	if len(sm.userSessions[session.UserEmail]) == 0 {
		delete(sm.userSessions, session.UserEmail)
	}
	
	sm.logger.WithField("session_id", sessionID).Debug("Session removed from tracking")
}

// GetSession retrieves session information
func (sm *SessionManager) GetSession(sessionID string) (*SessionInfo, error) {
	if !sm.config.Session.EnableSessionTracking {
		return nil, fmt.Errorf("session tracking not enabled")
	}
	
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	
	return session, nil
}

// GetUserSessions returns all sessions for a user
func (sm *SessionManager) GetUserSessions(userEmail string) []*SessionInfo {
	if !sm.config.Session.EnableSessionTracking {
		return nil
	}
	
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	sessionIDs := sm.userSessions[userEmail]
	sessions := make([]*SessionInfo, 0, len(sessionIDs))
	
	for _, sessionID := range sessionIDs {
		if session, exists := sm.sessions[sessionID]; exists {
			sessions = append(sessions, session)
		}
	}
	
	return sessions
}

// IsSessionNearExpiry checks if a session is near expiry
func (sm *SessionManager) IsSessionNearExpiry(sessionID string) bool {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return false
	}
	
	warningThreshold := time.Duration(sm.config.Session.ExpiryWarningMinutes) * time.Minute
	timeToExpiry := time.Until(session.ExpiresAt)
	
	return timeToExpiry > 0 && timeToExpiry <= warningThreshold
}

// RefreshSession extends the session expiry time
func (sm *SessionManager) RefreshSession(ctx context.Context, sessionID string) error {
	if !sm.config.Session.EnableAutoRefresh {
		return fmt.Errorf("auto refresh not enabled")
	}
	
	// Validate session in database
	valid, err := sm.db.ValidateSession(ctx, sessionID)
	if err != nil || !valid {
		return fmt.Errorf("session invalid or expired in EBS")
	}
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found in tracking")
	}
	
	// Extend expiry
	session.ExpiresAt = time.Now().Add(time.Duration(sm.config.Session.TimeoutMinutes) * time.Minute)
	session.LastActivity = time.Now()
	
	sm.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"new_expiry": session.ExpiresAt,
	}).Info("Session refreshed")
	
	return nil
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	interval := time.Duration(sm.config.Session.CleanupInterval) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sm.performCleanup()
		case <-sm.stopChan:
			sm.logger.Info("Stopping session cleanup")
			return
		}
	}
}

// performCleanup removes expired sessions
func (sm *SessionManager) performCleanup() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	now := time.Now()
	expiredCount := 0
	
	// Find expired sessions
	var expiredSessions []string
	for sessionID, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}
	
	// Remove expired sessions
	for _, sessionID := range expiredSessions {
		// Try to end session in EBS
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := sm.db.EndSession(ctx, sessionID); err != nil {
			sm.logger.WithError(err).WithField("session_id", sessionID).Warn("Failed to end session in EBS")
		}
		cancel()
		
		sm.removeSessionLocked(sessionID)
		expiredCount++
	}
	
	if expiredCount > 0 {
		sm.logger.WithFields(logrus.Fields{
			"expired_count":  expiredCount,
			"active_sessions": len(sm.sessions),
		}).Info("Cleaned up expired sessions")
	}
}

// GetStatistics returns session statistics
func (sm *SessionManager) GetStatistics() map[string]interface{} {
	if !sm.config.Session.EnableSessionTracking {
		return map[string]interface{}{
			"tracking_enabled": false,
		}
	}
	
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"tracking_enabled": true,
		"total_sessions":   len(sm.sessions),
		"total_users":      len(sm.userSessions),
	}
	
	// Count sessions near expiry
	nearExpiry := 0
	for _, session := range sm.sessions {
		warningThreshold := time.Duration(sm.config.Session.ExpiryWarningMinutes) * time.Minute
		timeToExpiry := time.Until(session.ExpiresAt)
		if timeToExpiry > 0 && timeToExpiry <= warningThreshold {
			nearExpiry++
		}
	}
	stats["sessions_near_expiry"] = nearExpiry
	
	return stats
}

// Stop stops the session manager
func (sm *SessionManager) Stop() {
	if sm.config.Session.EnableSessionTracking {
		close(sm.stopChan)
	}
}
