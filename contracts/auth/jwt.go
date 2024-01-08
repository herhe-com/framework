package auth

import "github.com/golang-jwt/jwt/v4"

const (
	JwtOfAuthorization = "Authorization"
)

type Claims struct {
	jwt.RegisteredClaims

	Refresh    bool           `json:"refresh,omitempty"`
	Platform   uint16         `json:"platform,omitempty"`
	PlatformID uint           `json:"platform_id,omitempty"`
	Ext        map[string]any `json:"ext,omitempty"`
}
