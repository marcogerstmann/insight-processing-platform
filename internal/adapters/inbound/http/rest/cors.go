package rest

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// corsMiddleware answers browser CORS preflight requests and adds the
// Access-Control-Allow-* headers for requests from an allowed origin.
//
// It exists only for the local runner (cmd/rest-local), where the web app is
// served from a different origin (the Vite dev server). In AWS, API Gateway owns
// CORS (see terraform/envs/dev/rest-api.tf), so this middleware is left unwired
// there — adding it would duplicate the Access-Control-Allow-Origin header,
// which browsers reject.
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && slices.Contains(allowedOrigins, origin) {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			// Signal caches that the response varies per origin, so one origin's
			// allow header is never served to another.
			header.Add("Vary", "Origin")
		}

		// Preflight requests carry no Authorization header, so they must be
		// answered before the auth middleware runs — mirroring how API Gateway
		// handles preflight ahead of the JWT authorizer.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
