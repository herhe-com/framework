package util

import "testing"

func TestValidPathNormalizesDirectoryPrefix(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: ""},
		{name: "leading slash", path: "/images", want: "images/"},
		{name: "relative prefix", path: "./images", want: "images/"},
		{name: "current directory", path: ".", want: ""},
		{name: "already normalized", path: "images/", want: "images/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidPath(tt.path); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
