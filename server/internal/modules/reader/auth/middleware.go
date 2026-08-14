package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const readerIdentityKey = "reader.identity"

func OptionalReader(service *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c.GetHeader("Authorization"))
		if raw != "" {
			if identity, err := service.ValidateToken(c.Request.Context(), raw); err == nil {
				c.Set(readerIdentityKey, identity)
			}
		}
		c.Next()
	}
}

func RequireReader(service *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c.GetHeader("Authorization"))
		if raw == "" {
			readerInvalidToken(c)
			c.Abort()
			return
		}
		identity, err := service.ValidateToken(c.Request.Context(), raw)
		if err != nil {
			readerInvalidToken(c)
			c.Abort()
			return
		}
		c.Set(readerIdentityKey, identity)
		c.Set("reader.token", raw)
		c.Next()
	}
}

func bearer(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func ReaderIdentity(c *gin.Context) (Identity, bool) {
	v, ok := c.Get(readerIdentityKey)
	identity, valid := v.(Identity)
	return identity, ok && valid
}
