package minio

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestUrlNormalizesDomainBucketAndFile(t *testing.T) {
	driver := &Minio{
		bucket: "assets",
		domain: "https://cdn.example.com/",
	}

	got := driver.Url("/images/logo.png")
	want := "https://cdn.example.com/assets/images/logo.png"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestUrlDoesNotAppendBucketTwice(t *testing.T) {
	driver := &Minio{
		bucket: "assets",
		domain: "https://cdn.example.com/assets",
	}

	got := driver.Url("images/logo.png")
	want := "https://cdn.example.com/assets/images/logo.png"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTemporaryUploadReturnsPresignedPutRequest(t *testing.T) {
	instance, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("key", "secret", ""),
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	driver := &Minio{
		ctx:      context.Background(),
		instance: instance,
		bucket:   "assets",
	}

	upload, err := driver.TemporaryUpload("/images/logo.png", 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if upload.Driver != "minio" {
		t.Fatalf("expected driver %q, got %q", "minio", upload.Driver)
	}

	if upload.Method != http.MethodPut {
		t.Fatalf("expected method %q, got %q", http.MethodPut, upload.Method)
	}

	if upload.Token != "" {
		t.Fatalf("expected empty token for presigned URL driver, got %q", upload.Token)
	}

	if !strings.Contains(upload.Url, "/assets/images/logo.png") {
		t.Fatalf("expected url to contain object key, got %q", upload.Url)
	}

	if upload.Key != "images/logo.png" {
		t.Fatalf("expected key %q, got %q", "images/logo.png", upload.Key)
	}

	if !strings.Contains(upload.Url, "X-Amz-Signature=") {
		t.Fatalf("expected url to contain a signature, got %q", upload.Url)
	}
}
