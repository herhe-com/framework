package filesystem

import (
	"io"
	"time"
)

type Storage interface {
	Driver
	Disk(disk string) (Driver, error)
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
	PresignedUploadUrl(file string, time time.Duration) (string, error)
	TemporaryUpload(file string, time time.Duration) (*TemporaryUpload, error)
	Url(file string) string
	// InitiateMultipartUpload begins a multipart upload and returns its upload ID.
	InitiateMultipartUpload(file string) (string, error)
	// UploadMultipartPart uploads one part of a multipart upload and returns the part ETag.
	UploadMultipartPart(file, uploadID string, number int64, content io.Reader, size int64) (string, error)
	// CompleteMultipartUpload assembles the uploaded parts into the final object.
	CompleteMultipartUpload(file, uploadID string, parts []MultipartPart) error
	// AbortMultipartUpload discards a multipart upload and its uploaded parts.
	AbortMultipartUpload(file, uploadID string) error
}

// MultipartPart identifies one uploaded part of a multipart upload.
type MultipartPart struct {
	Number int64
	ETag   string
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

// TemporaryUpload represents a temporary upload credential that allows a
// client to upload a file directly to the storage, without relaying the file
// content through the application server.
type TemporaryUpload struct {
	// Driver is the name of the storage driver that issued the credential,
	// e.g. "s3", "oss", "cos", "minio" or "qiniu".
	Driver string
	// Token is the temporary credential for token based drivers such as qiniu.
	// It is empty for presigned URL based drivers, whose credential is already
	// embedded in the Url and must not be sent separately.
	Token string
	// Url is the target URL the client sends the upload request to.
	Url string
	// Method is the HTTP method the client must use, e.g. "PUT" or "POST".
	Method string
	// Headers are extra HTTP headers the client should send with the request,
	// excluding Host, which is handled automatically by the HTTP client.
	Headers map[string]string
	// Fields are the form fields the client should include in a multipart form
	// upload, used by token based drivers, e.g. {"key": "...", "token": "..."}
	// for qiniu. It is empty for presigned URL based drivers.
	Fields map[string]string
	// Key is the object key the file will be stored under.
	Key string
	// ExpiresAt is the time at which the credential expires.
	ExpiresAt time.Time
}
