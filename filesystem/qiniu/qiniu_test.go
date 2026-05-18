package qiniu

import "testing"

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
