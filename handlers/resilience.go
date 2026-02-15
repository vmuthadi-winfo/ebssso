package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name             string
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	state            CircuitState
	failures         int
	successes        int
	lastFailureTime  time.Time
	mutex            sync.RWMutex
	logger           *logrus.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, cfg *config.ResilienceConfig, logger *logrus.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          time.Duration(cfg.CircuitTimeout) * time.Second,
		state:            CircuitClosed,
		logger:           logger,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}
	
	err := fn()
	
	if err != nil {
		cb.recordFailure()
		return err
	}
	
	cb.recordSuccess()
	return nil
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			cb.logger.WithField("circuit", cb.name).Info("Circuit breaker entering half-open state")
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	
	return false
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failures++
	cb.lastFailureTime = time.Now()
	
	if cb.state == CircuitHalfOpen {
		// Failure in half-open state, return to open
		cb.state = CircuitOpen
		cb.failures = 0
		cb.logger.WithField("circuit", cb.name).Warn("Circuit breaker reopened after failure")
		return
	}
	
	if cb.failures >= cb.failureThreshold {
		cb.state = CircuitOpen
		cb.logger.WithFields(logrus.Fields{
			"circuit":  cb.name,
			"failures": cb.failures,
		}).Warn("Circuit breaker opened")
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
			cb.logger.WithField("circuit", cb.name).Info("Circuit breaker closed after successful recovery")
		}
	} else if cb.state == CircuitClosed {
		// Reset failure count on success
		cb.failures = 0
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// ResilienceManager manages resilience features
type ResilienceManager struct {
	config          *config.Config
	logger          *logrus.Logger
	circuitBreakers map[string]*CircuitBreaker
	mutex           sync.RWMutex
}

// NewResilienceManager creates a new resilience manager
func NewResilienceManager(cfg *config.Config, logger *logrus.Logger) *ResilienceManager {
	rm := &ResilienceManager{
		config:          cfg,
		logger:          logger,
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
	
	// Initialize circuit breakers for critical dependencies
	if cfg.Resilience.EnableCircuitBreaker {
		rm.circuitBreakers["oidc"] = NewCircuitBreaker("oidc", &cfg.Resilience, logger)
		rm.circuitBreakers["database"] = NewCircuitBreaker("database", &cfg.Resilience, logger)
		rm.circuitBreakers["ebs"] = NewCircuitBreaker("ebs", &cfg.Resilience, logger)
	}
	
	return rm
}

// ExecuteWithCircuitBreaker executes a function with circuit breaker protection
func (rm *ResilienceManager) ExecuteWithCircuitBreaker(name string, fn func() error) error {
	if !rm.config.Resilience.EnableCircuitBreaker {
		return fn()
	}
	
	rm.mutex.RLock()
	cb, exists := rm.circuitBreakers[name]
	rm.mutex.RUnlock()
	
	if !exists {
		// Create circuit breaker on demand
		rm.mutex.Lock()
		cb = NewCircuitBreaker(name, &rm.config.Resilience, rm.logger)
		rm.circuitBreakers[name] = cb
		rm.mutex.Unlock()
	}
	
	return cb.Execute(fn)
}

// ExecuteWithRetry executes a function with retry logic
func (rm *ResilienceManager) ExecuteWithRetry(ctx context.Context, fn func() error) error {
	if !rm.config.Resilience.EnableRetry {
		return fn()
	}
	
	var lastErr error
	retryDelay := time.Duration(rm.config.Resilience.RetryDelay) * time.Second
	
	for attempt := 0; attempt <= rm.config.Resilience.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			default:
			}
			
			// Wait before retry with exponential backoff
			delay := retryDelay
			if rm.config.Resilience.RetryBackoff {
				delay = time.Duration(float64(retryDelay) * (float64(attempt)))
			}
			
			rm.logger.WithFields(logrus.Fields{
				"attempt": attempt,
				"delay":   delay,
			}).Debug("Retrying after failure")
			
			time.Sleep(delay)
		}
		
		err := fn()
		if err == nil {
			if attempt > 0 {
				rm.logger.WithField("attempt", attempt).Info("Operation succeeded after retry")
			}
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !rm.isRetryableError(err) {
			rm.logger.WithError(err).Debug("Error is not retryable")
			return err
		}
	}
	
	rm.logger.WithFields(logrus.Fields{
		"attempts": rm.config.Resilience.MaxRetries + 1,
		"error":    lastErr,
	}).Warn("All retry attempts failed")
	
	return fmt.Errorf("operation failed after %d attempts: %w", rm.config.Resilience.MaxRetries+1, lastErr)
}

// ExecuteWithTimeout executes a function with timeout
func (rm *ResilienceManager) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	errChan := make(chan error, 1)
	
	go func() {
		errChan <- fn(ctx)
	}()
	
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("operation timed out after %v: %w", timeout, ctx.Err())
	}
}

// isRetryableError determines if an error is retryable
func (rm *ResilienceManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := err.Error()
	
	// Network and temporary errors are retryable
	retryablePatterns := []string{
		"timeout",
		"temporary",
		"connection refused",
		"connection reset",
		"network unreachable",
		"no such host",
		"EOF",
		"broken pipe",
	}
	
	for _, pattern := range retryablePatterns {
		if containsIgnoreCase(errMsg, pattern) {
			return true
		}
	}
	
	return false
}

// containsIgnoreCase checks if a string contains a substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	var result []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}

// GetCircuitBreakerStates returns the state of all circuit breakers
func (rm *ResilienceManager) GetCircuitBreakerStates() map[string]string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	states := make(map[string]string)
	for name, cb := range rm.circuitBreakers {
		state := cb.GetState()
		switch state {
		case CircuitClosed:
			states[name] = "closed"
		case CircuitOpen:
			states[name] = "open"
		case CircuitHalfOpen:
			states[name] = "half-open"
		}
	}
	
	return states
}
