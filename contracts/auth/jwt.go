package auth

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	jwt.RegisteredClaims

	Type           string         `json:"typ,omitempty"`
	SessionID      string         `json:"sid,omitempty"`
	AccessLifetime int64          `json:"atl,omitempty"`
	Refresh        bool           `json:"ref,omitempty"` // Deprecated: use Type.
	Ext            map[string]any `json:"ext,omitempty"`
}

// TokenPair contains an access and refresh token pair.
type TokenPair struct {
	SessionID       string `json:"session_id"`
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	IssuedAt        int64  `json:"issued_at"`                // Unix seconds.
	AccessLifetime  int64  `json:"access_lifetime"`          // Seconds.
	RefreshLifetime int64  `json:"refresh_lifetime"`         // Seconds.
	GraceLifetime   int64  `json:"grace_lifetime,omitempty"` // Seconds.
}
