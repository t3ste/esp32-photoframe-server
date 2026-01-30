package middleware

import (
	"net/http"
	"strings"

	"github.com/aitjcize/photoframe-server/server/internal/service"
	"github.com/labstack/echo/v4"
)

func JWTMiddleware(authService *service.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract token - try multiple methods
			tokenString := extractToken(c)

			// Try HTTP Basic Authentication as alternative
			if tokenString == "" {
				if basicUser, basicPass, ok := c.Request().BasicAuth(); ok {
					if !authService.ValidateCredentials(basicUser, basicPass) {
						return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
					}
					// Generate a token for Basic Auth users
					tokenString, _ = authService.Login(basicUser, basicPass)
				}
			}

			if tokenString == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing authentication token"})
			}

			// Validate token
			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid authentication token"})
			}

			// Set user context
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)

			return next(c)
		}
	}
}

func extractToken(c echo.Context) string {
	authHeader := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	token := c.QueryParam("token")
	if token != "" {
		return token
	}

	return ""
}
