package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cobblepod/internal/auth"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/gin-gonic/gin"
)

// Auth0Middleware validates Auth0 JWT tokens using the official Auth0 middleware
func Auth0Middleware() gin.HandlerFunc {
	config := auth.GetAuth0Config()

	// Create JWKS provider with caching
	issuerURL, _ := url.Parse(fmt.Sprintf("https://%s/", config.Domain))
	provider := jwks.NewCachingProvider(issuerURL, 24*time.Hour)

	// Create JWT validator
	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{config.Audience},
	)
	if err != nil {
		// This should only happen during initialization with invalid config
		panic(fmt.Sprintf("Failed to create JWT validator: %v", err))
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		// Validate the token
		token, err := jwtValidator.ValidateToken(context.Background(), tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Invalid token: %v", err)})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.(*validator.ValidatedClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Store user ID and claims in context
		c.Set("user_id", claims.RegisteredClaims.Subject)
		c.Set("claims", claims)

		c.Next()
	}
}

// GetUserID is a helper to get user ID from context (use after Auth0Middleware)
func GetUserID(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("user not authenticated")
	}

	userIDStr, ok := userID.(string)
	if !ok {
		return "", fmt.Errorf("invalid user ID type")
	}

	return userIDStr, nil
}
