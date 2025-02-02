package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func LogrusMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != "/api/v1/health" {
			logger.WithFields(logrus.Fields{
				"user-agent": c.Request.UserAgent(),
				"error":      c.Errors.ByType(gin.ErrorTypePrivate).String(),
			}).Infof("[%s] %s: ", c.Request.Method, c.Request.URL.Path)
		}
		c.Next()
	}
}

// AuthMiddleware is the custom authentication middleware that checks the Authorization header to ensure a given
// discord id belong to a given refresh token.
func AuthMiddleware(cognito service.CognitoService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// Parse the Authorization header
		// Expected format: "Basic <base64-encoded discord_id:refresh_token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Basic" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Failed to decode credentials"})
			return
		}

		// Split the decoded credentials into Discord ID and refresh token
		credentials := strings.Split(string(decoded), ":")
		if len(credentials) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials format"})
			return
		}

		discordID := credentials[0]
		refreshToken := credentials[1]

		user, err := cognito.AuthUser(context.Background(), &refreshToken, &discordID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("could not authenticate user with refresh token: %s", err)})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}
