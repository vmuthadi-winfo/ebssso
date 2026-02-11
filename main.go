package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/vmuthadi-winfo/ebssso/config"
	"github.com/vmuthadi-winfo/ebssso/db"
	"github.com/vmuthadi-winfo/ebssso/handlers"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	logger.Info("Starting EBS SSO Gateway")

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Configure logger based on config
	configureLogger(logger, cfg)

	logger.WithField("environment", cfg.Environment).Info("Configuration loaded")

	// Initialize database connection
	oracleDB, err := db.NewOracleDB(&cfg.Database, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to Oracle database")
	}
	defer oracleDB.Close()

	// Initialize authentication handler
	authHandler, err := handlers.NewAuthHandler(cfg, oracleDB, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize authentication handler")
	}

	// Set Gin mode based on environment
	if cfg.Environment == "PROD" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(logger))

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	// Register routes
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/login")
	})
	router.GET("/login", authHandler.Login)
	router.GET("/callback", authHandler.Callback)
	router.GET("/logout", authHandler.Logout)
	router.GET("/health", authHandler.Health)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.WithField("address", addr).Info("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	logger.Info("EBS SSO Gateway is running")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

// configureLogger sets up logger based on configuration
func configureLogger(logger *logrus.Logger, cfg *config.Config) {
	// Set log level
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		logger.WithError(err).Warn("Invalid log level, defaulting to info")
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	if cfg.Logging.Format == "text" {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	// Set log output
	if cfg.Logging.Output != "" && cfg.Logging.Output != "stdout" {
		file, err := os.OpenFile(cfg.Logging.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logger.WithError(err).Error("Failed to open log file, using stdout")
		} else {
			logger.SetOutput(file)
		}
	}
}

// LoggerMiddleware is a Gin middleware for structured logging
func LoggerMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		fields := logrus.Fields{
			"status":     statusCode,
			"method":     method,
			"path":       path,
			"query":      query,
			"ip":         clientIP,
			"latency_ms": latency.Milliseconds(),
		}

		if len(c.Errors) > 0 {
			fields["errors"] = c.Errors.String()
		}

		entry := logger.WithFields(fields)

		if statusCode >= 500 {
			entry.Error("Server error")
		} else if statusCode >= 400 {
			entry.Warn("Client error")
		} else {
			entry.Info("Request completed")
		}
	}
}
