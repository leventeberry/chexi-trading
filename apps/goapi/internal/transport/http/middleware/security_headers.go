package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"goapi/config"
)

// permissionsPolicyConservative is a template-safe default: deny sensitive capabilities until a feature needs them.
const permissionsPolicyConservative = "camera=(), microphone=(), geolocation=(), payment=(), usb=(), display-capture=()"

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// SecurityHeaders sets baseline OWASP-style HTTP security headers when cfg.SecurityHeaders.Enabled is true.
// Headers are applied before c.Next() so route handlers may override specific values.
// When cfg is nil, the middleware is a no-op.
func SecurityHeaders(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || !cfg.SecurityHeaders.Enabled {
			c.Next()
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", permissionsPolicyConservative)
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		if cfg.SecurityHeaders.HSTSEnabled && requestIsHTTPS(c.Request) {
			v := fmt.Sprintf("max-age=%d", cfg.SecurityHeaders.HSTSMaxAgeSeconds)
			if cfg.SecurityHeaders.HSTSIncludeSubdomains {
				v += "; includeSubDomains"
			}
			c.Header("Strict-Transport-Security", v)
		}
		c.Next()
	}
}
