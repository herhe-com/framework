package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/herhe-com/framework/auth"
	"github.com/herhe-com/framework/facades"
)

func TestAuthRejectsConfiguredValidationError(t *testing.T) {
	registerJWTMiddlewareServices(t, nil)

	requestCtx := app.NewContext(0)
	requestCtx.Set(auth.ContextOfID, "user-1")
	called := false
	facades.Config().Set("auth.callback.auth", func(c context.Context, ctx *app.RequestContext) error {
		called = true
		if c == nil || auth.ID(ctx) != "user-1" {
			t.Fatal("expected callback to receive the authenticated request context")
		}

		return errors.New("account is disabled")
	})

	Auth()(context.Background(), requestCtx)

	if !called {
		t.Fatal("expected configured auth validation callback to be called")
	}
	if body := string(requestCtx.Response.Body()); !strings.Contains(body, "Unauthorized") {
		t.Fatalf("expected unauthorized response, got %q", body)
	}
}
