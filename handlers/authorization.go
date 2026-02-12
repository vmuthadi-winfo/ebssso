package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
)

// AuthorizationManager handles group-based access control
type AuthorizationManager struct {
	config *config.Config
	logger *logrus.Logger
}

// NewAuthorizationManager creates a new authorization manager
func NewAuthorizationManager(cfg *config.Config, logger *logrus.Logger) *AuthorizationManager {
	return &AuthorizationManager{
		config: cfg,
		logger: logger,
	}
}

// AuthorizeUser checks if a user is authorized based on their groups
func (am *AuthorizationManager) AuthorizeUser(ctx context.Context, userEmail string, groups []string) (bool, string) {
	return am.authorizeUser(ctx, userEmail, groups)
}

// authorizeUser checks if a user is authorized based on their groups (internal)
func (am *AuthorizationManager) authorizeUser(ctx context.Context, userEmail string, groups []string) (bool, string) {
	// If authorization is disabled, allow access
	if !am.config.Authorization.EnableAuthorization {
		am.logger.Debug("Authorization disabled, allowing access")
		return true, ""
	}
	
	// Log authorization attempt
	am.logger.WithFields(logrus.Fields{
		"user":   userEmail,
		"groups": groups,
	}).Info("Authorizing user")
	
	// Check denied groups first (takes precedence)
	if len(am.config.OIDC.DeniedGroups) > 0 {
		for _, deniedGroup := range am.config.OIDC.DeniedGroups {
			if am.isGroupMatch(groups, deniedGroup) {
				reason := fmt.Sprintf("User is in denied group: %s", deniedGroup)
				am.logAuthDecision(userEmail, false, reason)
				return false, reason
			}
		}
	}
	
	// If no allowed groups configured, allow by default (in permissive mode)
	if len(am.config.OIDC.AllowedGroups) == 0 {
		if am.config.Authorization.Mode == "permissive" {
			am.logger.Debug("No allowed groups configured, permissive mode - allowing access")
			return true, ""
		}
		// Strict mode requires explicit group configuration
		reason := "No allowed groups configured and strict mode enabled"
		am.logAuthDecision(userEmail, false, reason)
		return false, reason
	}
	
	// Check if user is in any allowed group
	if !am.config.OIDC.RequireGroupMatch {
		// Group match not required, allow access
		am.logger.Debug("Group match not required, allowing access")
		return true, ""
	}
	
	// Check group membership based on strategy
	matchedGroups := am.getMatchedGroups(groups, am.config.OIDC.AllowedGroups)
	
	if am.config.OIDC.GroupMatchStrategy == "all" {
		// User must be in ALL allowed groups
		if len(matchedGroups) == len(am.config.OIDC.AllowedGroups) {
			am.logAuthDecision(userEmail, true, fmt.Sprintf("User in all required groups: %v", matchedGroups))
			return true, ""
		}
		reason := fmt.Sprintf("User not in all required groups. Has: %v, Required: %v", matchedGroups, am.config.OIDC.AllowedGroups)
		am.logAuthDecision(userEmail, false, reason)
		return false, reason
	}
	
	// Default: "any" strategy - user must be in at least one allowed group
	if len(matchedGroups) > 0 {
		am.logAuthDecision(userEmail, true, fmt.Sprintf("User in allowed groups: %v", matchedGroups))
		return true, ""
	}
	
	reason := fmt.Sprintf("User not in any allowed groups. Has: %v, Required: %v", groups, am.config.OIDC.AllowedGroups)
	am.logAuthDecision(userEmail, false, reason)
	return false, reason
}

// isGroupMatch checks if any user group matches the pattern (supports wildcards)
func (am *AuthorizationManager) isGroupMatch(userGroups []string, pattern string) bool {
	for _, userGroup := range userGroups {
		if am.matchPattern(userGroup, pattern) {
			return true
		}
	}
	return false
}

// matchPattern performs pattern matching with wildcard support
func (am *AuthorizationManager) matchPattern(value, pattern string) bool {
	// Exact match
	if value == pattern {
		return true
	}
	
	// Wildcard at end: pattern/*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(value, prefix)
	}
	
	// Wildcard at start: */pattern
	if strings.HasPrefix(pattern, "*/") {
		suffix := strings.TrimPrefix(pattern, "*/")
		return strings.HasSuffix(value, suffix)
	}
	
	// Contains wildcard: *pattern*
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		substring := strings.Trim(pattern, "*")
		return strings.Contains(value, substring)
	}
	
	return false
}

// getMatchedGroups returns the list of user groups that match allowed groups
func (am *AuthorizationManager) getMatchedGroups(userGroups, allowedGroups []string) []string {
	var matched []string
	for _, allowedGroup := range allowedGroups {
		if am.isGroupMatch(userGroups, allowedGroup) {
			matched = append(matched, allowedGroup)
		}
	}
	return matched
}

// logAuthDecision logs the authorization decision
func (am *AuthorizationManager) logAuthDecision(userEmail string, allowed bool, reason string) {
	decision := "ALLOWED"
	if !allowed {
		decision = "DENIED"
	}
	
	fields := logrus.Fields{
		"user":     userEmail,
		"decision": decision,
		"reason":   reason,
	}
	
	if am.config.Authorization.EnableAuditLog {
		// Write to audit log
		am.logger.WithFields(fields).Info("Authorization decision")
	}
	
	if !allowed {
		am.logger.WithFields(fields).Warn("Access denied")
	} else {
		am.logger.WithFields(fields).Debug("Access granted")
	}
}

// ExtractGroups extracts groups from OIDC claims
func (am *AuthorizationManager) ExtractGroups(claims map[string]interface{}) []string {
	if am.config.OIDC.GroupClaim == "" {
		return []string{}
	}
	
	groupClaim, ok := claims[am.config.OIDC.GroupClaim]
	if !ok {
		am.logger.WithField("claim", am.config.OIDC.GroupClaim).Debug("Group claim not found in token")
		return []string{}
	}
	
	// Handle different group claim formats
	switch v := groupClaim.(type) {
	case []interface{}:
		// Array of groups
		groups := make([]string, 0, len(v))
		for _, g := range v {
			if groupStr, ok := g.(string); ok {
				groups = append(groups, groupStr)
			}
		}
		return groups
	case []string:
		// Direct string array
		return v
	case string:
		// Single group as string
		return []string{v}
	default:
		am.logger.WithFields(logrus.Fields{
			"claim": am.config.OIDC.GroupClaim,
			"type":  fmt.Sprintf("%T", v),
		}).Warn("Unexpected group claim format")
		return []string{}
	}
}

// AuthorizationMiddleware is a Gin middleware for authorization checks
func (am *AuthorizationManager) AuthorizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user email and groups from context (set by auth handler)
		userEmail, _ := c.Get("user_email")
		groups, _ := c.Get("user_groups")
		
		if userEmail == nil || groups == nil {
			am.logger.Warn("Missing user information in context")
			c.Next()
			return
		}
		
		userEmailStr := userEmail.(string)
		userGroups := groups.([]string)
		
		// Perform authorization check
		allowed, reason := am.authorizeUser(c.Request.Context(), userEmailStr, userGroups)
		
		if !allowed {
			if am.config.Authorization.OnFailureAction == "deny" {
				c.AbortWithStatusJSON(403, gin.H{
					"error":  "Access denied",
					"reason": reason,
				})
				return
			}
			// "log" mode - just log and continue
			am.logger.WithFields(logrus.Fields{
				"user":   userEmailStr,
				"reason": reason,
			}).Warn("Authorization failed but allowing access (log mode)")
		}
		
		c.Next()
	}
}
