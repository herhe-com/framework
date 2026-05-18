package s3

import "testing"

func TestUrlNormalizesDomainBucketAndFile(t *testing.T) {
	driver := &S3{
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
	driver := &S3{
		bucket: "assets",
		domain: "https://cdn.example.com/assets",
	}

	got := driver.Url("images/logo.png")
	want := "https://cdn.example.com/assets/images/logo.png"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
