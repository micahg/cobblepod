package endpoints

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWKS represents the JSON Web Key Set
type JWKS struct {
	Keys []JSONWebKey `json:"keys"`
}

// JSONWebKey represents a single key from JWKS
type JSONWebKey struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	Kid string   `json:"kid"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

var (
	jwksCache     *JWKS
	jwksCacheMux  sync.RWMutex
	jwksCacheTime time.Time
	jwksCacheTTL  = 24 * time.Hour
)

// Auth0Middleware validates Auth0 JWT tokens
func Auth0Middleware() gin.HandlerFunc {
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
		token, err := validateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Invalid token: %v", err)})
			c.Abort()
			return
		}

		// Extract claims and store in context
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["sub"])
			c.Set("claims", claims)
		}

		c.Next()
	}
}

// validateJWT validates a JWT token using Auth0's JWKS
func validateJWT(tokenString string) (*jwt.Token, error) {
	config := GetAuth0Config()

	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the kid from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid header not found")
		}

		// Get the public key from JWKS
		publicKey, err := getPublicKey(kid, config.Domain)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	// Validate claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify audience
	if config.Audience != "" {
		aud, ok := claims["aud"]
		if !ok {
			return nil, fmt.Errorf("missing audience claim")
		}
		// Audience can be string or []string
		switch v := aud.(type) {
		case string:
			if v != config.Audience {
				return nil, fmt.Errorf("invalid audience")
			}
		case []interface{}:
			found := false
			for _, a := range v {
				if audStr, ok := a.(string); ok && audStr == config.Audience {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("invalid audience")
			}
		default:
			return nil, fmt.Errorf("invalid audience format")
		}
	}

	// Verify issuer
	expectedIssuer := fmt.Sprintf("https://%s/", config.Domain)
	iss, ok := claims["iss"].(string)
	if !ok || iss != expectedIssuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Verify expiration
	exp, ok := claims["exp"].(float64)
	if !ok || time.Now().Unix() > int64(exp) {
		return nil, fmt.Errorf("token expired")
	}

	return token, nil
}

// getPublicKey retrieves the public key from Auth0's JWKS endpoint
func getPublicKey(kid, domain string) (*rsa.PublicKey, error) {
	jwks, err := getJWKS(domain)
	if err != nil {
		return nil, err
	}

	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			return parseRSAPublicKey(key)
		}
	}

	return nil, fmt.Errorf("key with kid %s not found", kid)
}

// getJWKS fetches the JWKS from Auth0 (with caching)
func getJWKS(domain string) (*JWKS, error) {
	// Check cache
	jwksCacheMux.RLock()
	if jwksCache != nil && time.Since(jwksCacheTime) < jwksCacheTTL {
		defer jwksCacheMux.RUnlock()
		return jwksCache, nil
	}
	jwksCacheMux.RUnlock()

	// Fetch from Auth0
	jwksCacheMux.Lock()
	defer jwksCacheMux.Unlock()

	url := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Update cache
	jwksCache = &jwks
	jwksCacheTime = time.Now()

	return &jwks, nil
}

// parseRSAPublicKey converts a JSONWebKey to an RSA public key
func parseRSAPublicKey(jwk JSONWebKey) (*rsa.PublicKey, error) {
	// Decode N (modulus)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N: %w", err)
	}

	// Decode E (exponent)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode E: %w", err)
	}

	// Convert to big.Int
	n := new(big.Int).SetBytes(nBytes)

	// Convert E bytes to int
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

// OptionalAuth0Middleware is like Auth0Middleware but doesn't reject requests without auth
func OptionalAuth0Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.Next()
			return
		}

		// Try to validate, but don't abort on error
		token, err := validateJWT(tokenString)
		if err == nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				c.Set("user_id", claims["sub"])
				c.Set("claims", claims)
			}
		}

		c.Next()
	}
}

// RequireAuth0 is a helper to get user ID from context (use after Auth0Middleware)
func RequireAuth0(c *gin.Context) (string, error) {
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
