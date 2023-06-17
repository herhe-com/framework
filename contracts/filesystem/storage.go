package filesystem

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	Driver
	Disk(disk string) Driver
}

type Driver interface {
	Dirs(path string) ([]Pathname, error)
	Files(path string) ([]Pathname, error)
	List(path string) ([]Pathname, error)
	Copy(oldFile, newFile string) error
	Delete(file ...string) error
	DeleteDirectory(directory string) error
	Exists(file string) bool
	MakeDirectory(directory string) error
	Missing(file string) bool
	Move(oldFile, newFile string) error
	Path(file string) string
	Put(file string, content io.Reader, size int64) error
	PutFile(path string, source File) (string, error)
	PutFileAs(path string, source File, name string) (string, error)
	Size(file string) (int64, error)
	TemporaryUrl(file string, time time.Duration) (string, error)
	WithContext(ctx context.Context) Driver
	Url(file string) string
}

type File interface {
	Disk(disk string) File
	File() string
	Store(path string) (string, error)
	StoreAs(path string, name string) (string, error)
	GetClientOriginalName() string
	GetClientOriginalExtension() string
	HashName(path ...string) string
	Extension() (string, error)
	Size() (int64, error)
}

type Pathname struct {
	Name  string
	Path  string
	IsDir bool
}
