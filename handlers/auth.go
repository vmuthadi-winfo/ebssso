package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
	"github.com/vmuthadi-winfo/ebssso/db"
	"golang.org/x/oauth2"
)

// AuthHandler handles OIDC authentication
type AuthHandler struct {
	config       *config.Config
	oauthConfig  *oauth2.Config
	oidcProvider *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	db           *db.OracleDB
	logger       *logrus.Logger
	states       map[string]time.Time // Simple state storage (in production, use Redis or similar)
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(cfg *config.Config, database *db.OracleDB, logger *logrus.Logger) (*AuthHandler, error) {
	ctx := context.Background()

	// Initialize OIDC provider
	provider, err := oidc.NewProvider(ctx, cfg.OIDC.ProviderURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	// Configure OAuth2
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL:  cfg.OIDC.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.OIDC.Scopes,
	}

	// Configure ID token verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.OIDC.ClientID,
	})

	return &AuthHandler{
		config:       cfg,
		oauthConfig:  oauthConfig,
		oidcProvider: provider,
		verifier:     verifier,
		db:           database,
		logger:       logger,
		states:       make(map[string]time.Time),
	}, nil
}

// generateState generates a random state string for CSRF protection
func (h *AuthHandler) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(b)
	
	// Store state with expiration
	h.states[state] = time.Now().Add(10 * time.Minute)
	
	// Clean up old states
	go h.cleanupStates()
	
	return state, nil
}

// validateState checks if the state is valid and not expired
func (h *AuthHandler) validateState(state string) bool {
	expiry, exists := h.states[state]
	if !exists {
		return false
	}
	
	if time.Now().After(expiry) {
		delete(h.states, state)
		return false
	}
	
	delete(h.states, state)
	return true
}

// cleanupStates removes expired states
func (h *AuthHandler) cleanupStates() {
	now := time.Now()
	for state, expiry := range h.states {
		if now.After(expiry) {
			delete(h.states, state)
		}
	}
}

// Login handles the /login route - redirects to OIDC provider
func (h *AuthHandler) Login(c *gin.Context) {
	state, err := h.generateState()
	if err != nil {
		h.logger.WithError(err).Error("Failed to generate state")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to initialize login",
		})
		return
	}

	h.logger.WithField("state", state).Info("Initiating OIDC login")

	// Redirect to OIDC provider's authorization endpoint
	authURL := h.oauthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusFound, authURL)
}

// Callback handles the /callback route - exchanges code for token
func (h *AuthHandler) Callback(c *gin.Context) {
	// Verify state for CSRF protection
	state := c.Query("state")
	if !h.validateState(state) {
		h.logger.Warn("Invalid or expired state")
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Invalid or expired state. Please try logging in again.",
		})
		return
	}

	// Handle OAuth2 errors
	if errMsg := c.Query("error"); errMsg != "" {
		errDesc := c.Query("error_description")
		h.logger.WithFields(logrus.Fields{
			"error":             errMsg,
			"error_description": errDesc,
		}).Error("OAuth2 error from provider")
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": fmt.Sprintf("Authentication failed: %s", errDesc),
		})
		return
	}

	// Exchange authorization code for tokens
	code := c.Query("code")
	if code == "" {
		h.logger.Error("No authorization code received")
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "No authorization code received",
		})
		return
	}

	ctx := c.Request.Context()
	oauth2Token, err := h.oauthConfig.Exchange(ctx, code)
	if err != nil {
		h.logger.WithError(err).Error("Failed to exchange code for token")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to complete authentication",
		})
		return
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		h.logger.Error("No id_token in OAuth2 token")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "No ID token received from provider",
		})
		return
	}

	// Verify ID token
	idToken, err := h.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		h.logger.WithError(err).Error("Failed to verify ID token")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to verify authentication token",
		})
		return
	}

	// Extract claims
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		h.logger.WithError(err).Error("Failed to parse claims")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to extract user information",
		})
		return
	}

	// Get user identifier from configured claim
	userIdentifier, ok := claims[h.config.OIDC.UserClaim].(string)
	if !ok {
		h.logger.WithField("claim", h.config.OIDC.UserClaim).Error("User claim not found")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("User claim '%s' not found in token", h.config.OIDC.UserClaim),
		})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"user_identifier": userIdentifier,
		"claim_type":      h.config.OIDC.UserClaim,
	}).Info("User authenticated via OIDC")

	// Look up user in EBS
	userName, err := h.db.GetUserByEmail(ctx, userIdentifier)
	if err != nil {
		h.logger.WithError(err).WithField("user_identifier", userIdentifier).Error("User not found in EBS")
		c.HTML(http.StatusUnauthorized, "error.html", gin.H{
			"error": fmt.Sprintf("User '%s' not found in EBS. Please contact your administrator.", userIdentifier),
		})
		return
	}

	// Create EBS session
	sessionID, err := h.db.CreateEBSSession(ctx, userName)
	if err != nil {
		h.logger.WithError(err).WithField("user_name", userName).Error("Failed to create EBS session")
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to create EBS session. Please try again or contact your administrator.",
		})
		return
	}

	// Set EBS cookies
	h.setEBSCookies(c, sessionID)

	h.logger.WithFields(logrus.Fields{
		"user_name":  userName,
		"session_id": sessionID,
	}).Info("Successfully created EBS session, redirecting to EBS")

	// Redirect to EBS home page
	ebsURL := h.config.EBS.BaseURL + h.config.EBS.HomePage
	c.Redirect(http.StatusFound, ebsURL)
}

// setEBSCookies sets the required EBS session cookies
func (h *AuthHandler) setEBSCookies(c *gin.Context, sessionID string) {
	// Set ICX_SESSION cookie (primary session cookie)
	c.SetCookie(
		"ICX_SESSION",
		sessionID,
		h.config.Session.TimeoutMinutes*60, // maxAge in seconds
		"/",
		h.config.EBS.CookieDomain,
		h.config.EBS.CookieSecure,
		true, // httpOnly
	)

	// Set additional EBS session cookie
	c.SetCookie(
		"ebs_session",
		sessionID,
		h.config.Session.TimeoutMinutes*60,
		"/",
		h.config.EBS.CookieDomain,
		h.config.EBS.CookieSecure,
		true,
	)

	// Set our own SSO session cookie for tracking
	c.SetCookie(
		h.config.Session.CookieName,
		sessionID,
		h.config.Session.TimeoutMinutes*60,
		"/",
		h.config.EBS.CookieDomain,
		h.config.EBS.CookieSecure,
		true,
	)

	h.logger.WithField("session_id", sessionID).Debug("Set EBS session cookies")
}

// Logout handles the /logout route
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get session ID from cookie
	sessionID, err := c.Cookie(h.config.Session.CookieName)
	if err == nil && sessionID != "" {
		ctx := c.Request.Context()
		if err := h.db.EndSession(ctx, sessionID); err != nil {
			h.logger.WithError(err).Error("Failed to end session in database")
		}
	}

	// Clear cookies
	h.clearEBSCookies(c)

	h.logger.Info("User logged out")

	c.HTML(http.StatusOK, "logout.html", gin.H{
		"message": "You have been successfully logged out.",
	})
}

// clearEBSCookies clears all EBS session cookies
func (h *AuthHandler) clearEBSCookies(c *gin.Context) {
	cookies := []string{"ICX_SESSION", "ebs_session", h.config.Session.CookieName}
	
	for _, cookie := range cookies {
		c.SetCookie(
			cookie,
			"",
			-1, // negative maxAge to delete
			"/",
			h.config.EBS.CookieDomain,
			h.config.EBS.CookieSecure,
			true,
		)
	}
}

// Health handles the /health route for health checks
func (h *AuthHandler) Health(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Check database connection
	_, err := h.db.ValidateSession(ctx, "0")
	dbHealthy := err == nil || err.Error() != "connection error"

	status := "healthy"
	httpStatus := http.StatusOK
	
	if !dbHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":   status,
		"database": dbHealthy,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
