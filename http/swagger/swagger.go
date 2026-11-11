// Package swagger 提供带深色主题的 Swagger UI HTTP 处理器。
package swagger

import (
	_ "embed"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

// theme 是 Swagger UI 深色主题脚本：注入样式表、跟随系统偏好并持久化用户手动切换。
//
//go:embed theme.js
var theme string

// Handler 返回 Swagger UI 的 HTTP 处理器，文档接口默认读取 doc.json。
func Handler() http.HandlerFunc {
	return httpSwagger.Handler(
		httpSwagger.UIConfig(map[string]string{
			"defaultModelsExpandDepth": "-1",
			"showExtensions":           "true",
		}),
		httpSwagger.BeforeScript(theme),
	)
}
