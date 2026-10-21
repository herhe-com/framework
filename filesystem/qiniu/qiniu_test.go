package qiniu

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUrlNormalizesDomainPrefixAndFile(t *testing.T) {
	driver := &Qiniu{
		domain:    "https://cdn.example.com/",
		prefix:    "uploads/",
		delimiter: "/",
	}

	got := driver.Url("/images/logo.png")
	want := "https://cdn.example.com/uploads/images/logo.png"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestUrlDoesNotAppendPrefixTwice(t *testing.T) {
	driver := &Qiniu{
		domain:    "https://cdn.example.com",
		prefix:    "uploads/",
		delimiter: "/",
	}

	got := driver.Url("uploads/images/logo.png")
	want := "https://cdn.example.com/uploads/images/logo.png"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTemporaryUploadReturnsTokenAndUploadEndpoint(t *testing.T) {
	driver := &Qiniu{
		access:    "access-key",
		secret:    "secret-key",
		bucket:    "assets",
		uploadUrl: "https://up.example.com",
		prefix:    "uploads/",
		delimiter: "/",
	}

	upload, err := driver.TemporaryUpload("/images/logo.png", 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if upload.Driver != "qiniu" {
		t.Fatalf("expected driver %q, got %q", "qiniu", upload.Driver)
	}

	if upload.Method != http.MethodPost {
		t.Fatalf("expected method %q, got %q", http.MethodPost, upload.Method)
	}

	if upload.Url != "https://up.example.com" {
		t.Fatalf("expected url %q, got %q", "https://up.example.com", upload.Url)
	}

	if upload.Token == "" || !strings.HasPrefix(upload.Token, "access-key:") {
		t.Fatalf("expected token issued by the access key, got %q", upload.Token)
	}

	wantKey := "uploads/images/logo.png"
	if upload.Key != wantKey {
		t.Fatalf("expected key %q, got %q", wantKey, upload.Key)
	}

	if upload.Fields["key"] != wantKey {
		t.Fatalf("expected field key %q, got %q", wantKey, upload.Fields["key"])
	}

	if upload.Fields["token"] != upload.Token {
		t.Fatalf("expected field token to match the credential token")
	}
}
