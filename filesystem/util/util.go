package util

import (
	"path"
	"strings"

	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/support/file"
)

const MaxFileNum = 100

func FullPathOfFile(filePath string, source filesystem.File, name string) (string, error) {
	extension := path.Ext(name)
	if extension == "" {
		var err error
		extension, err = file.Extension(source.File(), true)
		if err != nil {
			return "", err
		}

		return strings.TrimSuffix(filePath, "/") + "/" + strings.TrimSuffix(strings.TrimPrefix(path.Base(name), "/"), "/") + "." + extension, nil
	} else {
		return strings.TrimSuffix(filePath, "/") + "/" + strings.TrimPrefix(path.Base(name), "/"), nil
	}
}

func ValidPath(path string) string {
	realPath := strings.TrimPrefix(path, "./")
	realPath = strings.TrimPrefix(realPath, "/")
	realPath = strings.TrimPrefix(realPath, ".")
	if realPath != "" && !strings.HasSuffix(realPath, "/") {
		realPath += "/"
	}

	return realPath
}
