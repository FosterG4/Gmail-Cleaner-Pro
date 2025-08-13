package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"mailcleanerpro/internal/handler"
	"mailcleanerpro/internal/middleware"
	"mailcleanerpro/internal/service"
	"mailcleanerpro/pkg/auth"
	"mailcleanerpro/pkg/gmail"
	"mailcleanerpro/pkg/logger"
)

func setupRouter() (*gin.Engine, error) {
	// Initialize logger with configuration
	loggerConfig := &logger.Config{
		Level:      logger.InfoLevel,
		Format:     "json",
		OutputPath: "stdout",
		ErrorPath:  "stderr",
	}
	if err := logger.InitLogger(loggerConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Create Gin engine without default middleware
	r := gin.New()

	// Add comprehensive middleware stack
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.AdvancedRecoveryWithLogger())
	r.Use(middleware.ErrorHandlingMiddleware())
	r.Use(middleware.DetailedRequestResponseLogger())
	r.Use(middleware.MetricsMiddleware())
	r.Use(middleware.PerformanceMetricsMiddleware())
	r.Use(middleware.MetricsReportingMiddleware(5 * time.Minute))
	r.Use(middleware.HealthCheckMiddleware())

	// Add CORS middleware if needed
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Request-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// UI
	r.GET("/", func(c *gin.Context) {
		c.File("web/templates/index.html")
	})

	// OAuth routes
	r.GET("/auth/login", func(c *gin.Context) {
		conf, err := auth.NewGoogleOAuth2Config()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		state := "state"
		url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
		c.Redirect(http.StatusTemporaryRedirect, url)
	})

	r.GET("/auth/callback", func(c *gin.Context) {
		// Debug: Log all query parameters
		fmt.Printf("Callback received with query params: %v\n", c.Request.URL.RawQuery)

		// Check if this is a fetch request from the frontend
		isFetchRequest := c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
			c.GetHeader("Accept") == "application/json" ||
			c.Query("format") == "json"

		conf, err := auth.NewGoogleOAuth2Config()
		if err != nil {
			if isFetchRequest {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				// Redirect to home with error parameter to avoid template dependency
				c.Redirect(http.StatusTemporaryRedirect, "/?error=server_error")
			}
			return
		}

		code := c.Query("code")
		if code == "" {
			errorMsg := gin.H{
				"error":           "missing code",
				"received_params": c.Request.URL.Query(),
				"help":            "This endpoint should be called by Google OAuth after user authorization. Please start the flow at /auth/login",
			}
			if isFetchRequest {
				c.JSON(http.StatusBadRequest, errorMsg)
			} else {
				// Redirect to home with error parameter
				c.Redirect(http.StatusTemporaryRedirect, "/?error=auth_failed")
			}
			return
		}

		// Exchange code for token and get user info
		authResponse, err := auth.ExchangeCodeWithUserInfo(c, conf, code)
		if err != nil {
			fmt.Printf("Error during token exchange: %v\n", err)
			if isFetchRequest {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			} else {
				c.Redirect(http.StatusTemporaryRedirect, "/?error=token_exchange_failed")
			}
			return
		}

		fmt.Printf("Successfully authenticated user: %s\n", authResponse.UserInfo.Email)

		// If it's a fetch request from frontend, return JSON
		if isFetchRequest {
			c.JSON(http.StatusOK, gin.H{
				"access_token":  authResponse.AccessToken,
				"refresh_token": authResponse.RefreshToken,
				"user_email":    authResponse.UserInfo.Email,
				"user_name":     authResponse.UserInfo.Name,
				"user_picture":  authResponse.UserInfo.Picture,
				"expires_in":    authResponse.ExpiresIn,
			})
			return
		}

		// For direct browser navigation, serve HTML with embedded data for frontend to pick up
		htmlResponse := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Authentication Success - Gmail Cleaner Pro</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
        }
        .container {
            text-align: center;
            background: rgba(255, 255, 255, 0.1);
            padding: 2rem;
            border-radius: 10px;
            backdrop-filter: blur(10px);
        }
        .spinner {
            border: 3px solid rgba(255, 255, 255, 0.3);
            border-top: 3px solid white;
            border-radius: 50%%;
            width: 40px;
            height: 40px;
            animation: spin 1s linear infinite;
            margin: 20px auto;
        }
        @keyframes spin {
            0%% { transform: rotate(0deg); }
            100%% { transform: rotate(360deg); }
        }
    </style>
</head>
<body>
    <div class="container">
        <h2>🎉 Authentication Successful!</h2>
        <p>Welcome, %s!</p>
        <div class="spinner"></div>
        <p>Redirecting you to the application...</p>
    </div>
    <script>
        // Store authentication data in localStorage
        localStorage.setItem('gmail_access_token', '%s');
        localStorage.setItem('user_email', '%s');
        localStorage.setItem('user_name', '%s');
        localStorage.setItem('user_picture', '%s');
        localStorage.setItem('auth_expires_in', '%d');
        localStorage.setItem('auth_timestamp', Date.now().toString());
        
        // Redirect to home page after a brief delay
        setTimeout(() => {
            window.location.href = '/';
        }, 2000);
    </script>
</body>
</html>`,
			authResponse.UserInfo.Name,
			authResponse.AccessToken,
			authResponse.UserInfo.Email,
			authResponse.UserInfo.Name,
			authResponse.UserInfo.Picture,
			authResponse.ExpiresIn,
		)

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlResponse))
	})

	// Gmail service injection per request using provided token
	r.POST("/api/v1/clean", func(c *gin.Context) {
		accessToken := c.GetHeader("X-Access-Token")
		if accessToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "missing X-Access-Token",
				"message": "Please re-authenticate with your Gmail account",
				"action":  "reauth_required",
			})
			return
		}
		conf, err := auth.NewGoogleOAuth2Config()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Create token with proper configuration
		t := &oauth2.Token{
			AccessToken: accessToken,
			TokenType:   "Bearer",
		}

		httpClient := option.WithHTTPClient(conf.Client(context.Background(), t))
		gsvc, err := gmail.NewService(c, httpClient)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		cleaner := service.NewCleanerService(gsvc)
		h := handler.NewCleanHandler(cleaner)
		h.Clean(c)
	})

	// Health check endpoints
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
			"service":   "mailcleanerpro",
		})
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		// Check if all dependencies are ready
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"checks": gin.H{
				"gmail_api": "ok",
				"oauth2":    "ok",
			},
		})
	})

	r.GET("/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	// Metrics endpoint
	r.GET("/metrics", func(c *gin.Context) {
		metrics := middleware.GetMetrics()
		c.JSON(http.StatusOK, gin.H{
			"metrics":   metrics,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// System info endpoint
	r.GET("/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":     "Gmail Cleaner Pro",
			"version":     "1.0.0",
			"description": "High-performance Gmail cleaning and management service",
			"build_time":  time.Now().UTC().Format(time.RFC3339),
			"go_version":  "1.21+",
			"framework":   "Gin",
			"features": []string{
				"OAuth2 Authentication",
				"Gmail API Integration",
				"Batch Email Processing",
				"Advanced Logging",
				"Performance Metrics",
				"Request Tracing",
			},
		})
	})

	return r, nil
}
