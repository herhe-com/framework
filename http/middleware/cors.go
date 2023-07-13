package middleware

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/cors"
)

func Cors() app.HandlerFunc {

	return cors.New(cors.Config{
		AllowAllOrigins: true,
	})
}
