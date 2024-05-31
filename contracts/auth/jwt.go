package auth

import "github.com/golang-jwt/jwt/v4"

const (
	JwtOfAuthorization = "Authorization"
)

type Claims struct {
	jwt.RegisteredClaims

	Refresh      bool           `json:"ref,omitempty"`
	Platform     uint16         `json:"plt,omitempty"`
	Organization *string        `json:"org,omitempty"`
	Clique       *string        `json:"clq,omitempty"`
	Ext          map[string]any `json:"ext,omitempty"`
}
