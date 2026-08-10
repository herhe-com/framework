package middleware

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/cors"

	"github.com/herhe-com/framework/auth"
)

func Cors() app.HandlerFunc {

	return cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", auth.JwtOfAuthorization, auth.JwtOfRefreshToken},
		ExposeHeaders:    []string{auth.JwtOfTokenPair},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
}
