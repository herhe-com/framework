package auth

import "github.com/golang-jwt/jwt/v5"

const (
	JwtOfAuthorization = "Authorization"
)

type Claims struct {
	jwt.RegisteredClaims

	Refresh bool           `json:"ref,omitempty"`
	Ext     map[string]any `json:"ext,omitempty"`
}
