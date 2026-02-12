package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
	"github.com/vmuthadi-winfo/ebssso/db"
)

// EBSProxyHandler handles proxying authenticated requests to EBS
type EBSProxyHandler struct {
	config *config.Config
	db     *db.OracleDB
	logger *logrus.Logger
	client *http.Client
}

// NewEBSProxyHandler creates a new EBS proxy handler
func NewEBSProxyHandler(cfg *config.Config, database *db.OracleDB, logger *logrus.Logger) *EBSProxyHandler {
	return &EBSProxyHandler{
		config: cfg,
		db:     database,
		logger: logger,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects, pass them through to client
				return http.ErrUseLastResponse
			},
		},
	}
}

// ProxyRequest proxies an authenticated request to EBS
func (p *EBSProxyHandler) ProxyRequest(c *gin.Context) {
	// Validate session
	sessionID, err := c.Cookie(p.config.Session.CookieName)
	if err != nil || sessionID == "" {
		// No session, redirect to login with return URL
		returnURL := p.config.EBS.BaseURL + c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			returnURL += "?" + c.Request.URL.RawQuery
		}
		
		loginURL := fmt.Sprintf("/login?return_url=%s", url.QueryEscape(returnURL))
		c.Redirect(http.StatusFound, loginURL)
		return
	}
	
	// Validate session in database
	ctx := c.Request.Context()
	valid, err := p.db.ValidateSession(ctx, sessionID)
	if err != nil || !valid {
		p.logger.WithFields(logrus.Fields{
			"session_id": sessionID,
			"error":      err,
		}).Warn("Invalid or expired session")
		
		// Session invalid, redirect to login
		returnURL := p.config.EBS.BaseURL + c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			returnURL += "?" + c.Request.URL.RawQuery
		}
		
		loginURL := fmt.Sprintf("/login?return_url=%s", url.QueryEscape(returnURL))
		c.Redirect(http.StatusFound, loginURL)
		return
	}
	
	// Build target URL
	targetURL := p.config.EBS.BaseURL + c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}
	
	// Check if path is allowed
	if !p.isPathAllowed(c.Request.URL.Path) {
		p.logger.WithField("path", c.Request.URL.Path).Warn("Path not allowed for proxy")
		c.JSON(http.StatusForbidden, gin.H{"error": "Path not allowed"})
		return
	}
	
	// Create proxy request
	proxyReq, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		p.logger.WithError(err).Error("Failed to create proxy request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy request"})
		return
	}
	
	// Copy headers
	for key, values := range c.Request.Header {
		// Skip certain headers
		if key == "Host" || strings.HasPrefix(key, "X-Forwarded-") {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}
	
	// Copy EBS session cookies
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == "ICX_SESSION" || cookie.Name == "ebs_session" {
			proxyReq.AddCookie(cookie)
		}
	}
	
	// Execute proxy request
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		p.logger.WithError(err).Error("Failed to execute proxy request")
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to reach EBS"})
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	
	// Copy response status and body
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
	
	p.logger.WithFields(logrus.Fields{
		"method":      c.Request.Method,
		"path":        c.Request.URL.Path,
		"status":      resp.StatusCode,
		"session_id":  sessionID,
	}).Debug("Proxied request to EBS")
}

// isPathAllowed checks if a path is allowed for proxying
func (p *EBSProxyHandler) isPathAllowed(path string) bool {
	// If no allowed paths configured, allow all
	if len(p.config.EBS.AllowedPaths) == 0 {
		return true
	}
	
	// Check against allowed paths
	for _, pattern := range p.config.EBS.AllowedPaths {
		if matchesPattern(path, pattern) {
			return true
		}
	}
	
	return false
}

// ValidateSession middleware validates EBS session before processing request
func (p *EBSProxyHandler) ValidateSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(p.config.Session.CookieName)
		if err != nil || sessionID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
			return
		}
		
		ctx := c.Request.Context()
		valid, err := p.db.ValidateSession(ctx, sessionID)
		if err != nil || !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			return
		}
		
		// Store session ID in context for handlers
		c.Set("session_id", sessionID)
		c.Next()
	}
}
