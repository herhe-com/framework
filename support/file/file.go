package file

import (
	"errors"
	"os"
	"path"
	"strings"

	"github.com/h2non/filetype"
)

// Extension Supported types: https://github.com/h2non/filetype#supported-types
func Extension(file string, originalWhenUnknown ...bool) (string, error) {
	buf, _ := os.ReadFile(file)
	kind, err := filetype.Match(buf)
	if err != nil {
		return "", err
	}

	if kind == filetype.Unknown {
		if len(originalWhenUnknown) > 0 {
			if originalWhenUnknown[0] {
				return ClientOriginalExtension(file), nil
			}
		}

		return "", errors.New("unknown file extension")
	}

	return kind.Extension, nil
}

func ClientOriginalExtension(file string) string {
	return strings.ReplaceAll(path.Ext(file), ".", "")
}
