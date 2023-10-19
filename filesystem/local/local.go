package local

import (
	"bytes"
	"errors"
	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/filesystem/util"
	"github.com/herhe-com/framework/support/str"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

type Local struct {
	root   string
	domain string
}

func NewLocal() (*Local, error) {
	return &Local{
		root:   facades.Cfg.GetString("filesystems.disks.local.root"),
		domain: facades.Cfg.GetString("filesystems.disks.local.url"),
	}, nil
}

func (local *Local) Dirs(path string) (dirs []filesystem.Pathname, err error) {

	fileInfo, _ := os.ReadDir(local.fullPath(path))

	for _, f := range fileInfo {
		if f.IsDir() {
			dirs = append(dirs, filesystem.Pathname{
				Name:  f.Name(),
				Path:  f.Name() + "/",
				IsDir: true,
			})
		}
	}

	return dirs, nil
}

func (local *Local) Files(path string) (files []filesystem.Pathname, err error) {

	fileInfo, err := os.ReadDir(local.fullPath(path))

	if err != nil {
		return nil, err
	}
	for _, f := range fileInfo {
		if !f.IsDir() {
			files = append(files, filesystem.Pathname{
				Name:  f.Name(),
				Path:  f.Name(),
				IsDir: false,
			})
		}
	}

	return files, err
}

func (local *Local) List(path string) (list []filesystem.Pathname, err error) {

	fileInfo, err := os.ReadDir(local.fullPath(path))

	if err != nil {
		return nil, err
	}

	for _, f := range fileInfo {
		list = append(list, filesystem.Pathname{
			Name:  f.Name(),
			Path:  f.Name(),
			IsDir: f.IsDir(),
		})
	}

	return list, err
}

func (local *Local) Copy(originFile, targetFile string) error {

	file, err := os.ReadFile(local.fullPath(originFile))

	if err != nil {
		return err
	}

	reader := bytes.NewReader(file)

	return local.Put(targetFile, reader, reader.Size())
}

func (local *Local) Delete(files ...string) error {
	for _, file := range files {
		fileInfo, err := os.Stat(local.fullPath(file))
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			return errors.New("can't delete directory, please use DeleteDirectory")
		}
	}

	for _, file := range files {
		if err := os.Remove(local.fullPath(file)); err != nil {
			return err
		}
	}

	return nil
}

func (local *Local) DeleteDirectory(directory string) error {
	return os.RemoveAll(local.fullPath(directory))
}

func (local *Local) Exists(file string) bool {
	_, err := os.Stat(local.fullPath(file))
	if err != nil {
		return os.IsExist(err)
	}
	return true
}

func (local *Local) Get(file string) (string, error) {
	data, err := os.ReadFile(local.fullPath(file))
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (local *Local) MakeDirectory(directory string) error {
	return os.MkdirAll(path.Dir(local.fullPath(directory)+"/"), os.ModePerm)
}

func (local *Local) Missing(file string) bool {
	return !local.Exists(file)
}

func (local *Local) Move(oldFile, newFile string) error {
	newFile = local.fullPath(newFile)
	if err := os.MkdirAll(path.Dir(newFile), os.ModePerm); err != nil {
		return err
	}

	if err := os.Rename(local.fullPath(oldFile), newFile); err != nil {
		return err
	}

	return nil
}

func (local *Local) Path(file string) string {
	return facades.Root + "/" + strings.TrimPrefix(strings.TrimPrefix(local.fullPath(file), "/"), "./")
}

func (local *Local) Put(key string, file io.Reader, size int64) error {

	key = local.fullPath(key)

	if err := os.MkdirAll(path.Dir(key), os.ModePerm); err != nil {
		return err
	}

	f, err := os.Create(key)
	if err != nil {
		return err
	}

	defer f.Close()

	buf := new(bytes.Buffer)
	if _, err = buf.ReadFrom(file); err != nil {
		return err
	}

	if _, err = f.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (local *Local) PutFile(filePath string, source filesystem.File) (string, error) {
	return local.PutFileAs(filePath, source, str.Random(40))
}

func (local *Local) PutFileAs(filePath string, source filesystem.File, name string) (string, error) {

	data, err := os.ReadFile(source.File())
	if err != nil {
		return "", err
	}

	fullPath, err := util.FullPathOfFile(filePath, source, name)
	if err != nil {
		return "", err
	}

	reader := bytes.NewReader(data)

	if err := local.Put(fullPath, reader, reader.Size()); err != nil {
		return "", err
	}

	return fullPath, nil
}

func (local *Local) Size(file string) (int64, error) {
	fileInfo, err := os.Open(local.fullPath(file))
	if err != nil {
		return 0, err
	}

	fi, err := fileInfo.Stat()
	if err != nil {
		return 0, err
	}

	return fi.Size(), nil
}

func (local *Local) TemporaryUrl(file string, time time.Duration) (string, error) {
	return local.Url(file), nil
}

func (local *Local) Url(file string) string {
	return strings.TrimSuffix(local.domain, "/") + "/" + strings.TrimPrefix(file, "/")
}

func (local *Local) fullPath(path string) string {
	if path == "." {
		path = ""
	}
	realPath := strings.TrimPrefix(path, "./")
	realPath = strings.TrimSuffix(strings.TrimPrefix(realPath, "/"), "/")
	if realPath == "" {
		return local.rootPath()
	} else {
		return local.rootPath() + realPath
	}
}

func (local *Local) rootPath() string {
	return strings.TrimSuffix(local.root, "/") + "/"
}
