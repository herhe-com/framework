package oss

import (
	"net/http"
	"reflect"
	"testing"
)

func TestUrlNormalizesDomainBucketAndFile(t *testing.T) {
	driver := &OSS{
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
	driver := &OSS{
		bucket: "assets",
		domain: "https://cdn.example.com/assets",
	}

	got := driver.Url("images/logo.png")
	want := "https://cdn.example.com/assets/images/logo.png"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSignedHeadersExcludesHostAndCanonicalizesNames(t *testing.T) {
	header := http.Header{}
	header.Set("Host", "cdn.example.com")
	header.Set("X-Amz-Meta-Foo", "bar")

	got := signedHeaders(header)

	want := map[string]string{"X-Amz-Meta-Foo": "bar"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
