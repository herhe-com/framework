package minio

import (
	"bytes"
	"context"
	"fmt"
	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/filesystem/util"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

/*
 * MinIO OSS
 * Document: https://min.io/docs/minio/linux/developers/go/minio-go.html
 * Example: https://github.com/minio/minio-go/tree/master/examples/s3
 */

type Minio struct {
	ctx      context.Context
	instance *minio.Client
	bucket   string
	disk     string
	domain   string
}

func NewMinio(ctx context.Context, configs map[string]any) (*Minio, error) {

	cfg := viper.New()

	cfg.Set("minio", configs)

	cfg.SetDefault("minio.ssl", false)

	key := cfg.GetString("minio.key")
	secret := cfg.GetString("minio.secret")
	region := cfg.GetString("minio.region")
	bucket := cfg.GetString("minio.bucket")
	domain := cfg.GetString("minio.domain")
	ssl := cfg.GetBool("minio.ssl")
	endpoint := cfg.GetString("minio.endpoint")

	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(key, secret, ""),
		Secure: ssl,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio disk error: %v", err)
	}

	return &Minio{
		ctx:      ctx,
		instance: client,
		bucket:   bucket,
		domain:   domain,
	}, nil
}

func (r *Minio) Dirs(path string) (dirs []filesystem.Pathname, err error) {

	prefix := util.ValidPath(path)

	objectCh := r.instance.ListObjects(r.ctx, r.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		if strings.HasSuffix(object.Key, "/") {
			dirs = append(dirs, filesystem.Pathname{
				Name:  strings.ReplaceAll(object.Key, prefix, ""),
				Path:  object.Key,
				IsDir: true,
			})
		}
	}

	return dirs, nil
}

func (r *Minio) Files(path string) (files []filesystem.Pathname, err error) {

	prefix := util.ValidPath(path)

	objectCh := r.instance.ListObjects(r.ctx, r.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		if !strings.HasSuffix(object.Key, "/") {
			files = append(files, filesystem.Pathname{
				Name:  strings.TrimPrefix(object.Key, prefix),
				Path:  object.Key,
				IsDir: false,
			})
		}
	}

	return files, nil
}

func (r *Minio) List(path string) (list []filesystem.Pathname, err error) {

	prefix := util.ValidPath(path)

	objectCh := r.instance.ListObjects(r.ctx, r.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}

		isDir := strings.HasSuffix(object.Key, "/")

		list = append(list, filesystem.Pathname{
			Name:  strings.ReplaceAll(object.Key, prefix, ""),
			Path:  object.Key,
			IsDir: isDir,
		})
	}

	return list, nil
}

func (r *Minio) Copy(originFile, targetFile string) error {
	srcOpts := minio.CopySrcOptions{
		Bucket: r.bucket,
		Object: originFile,
	}
	dstOpts := minio.CopyDestOptions{
		Bucket: r.bucket,
		Object: targetFile,
	}
	_, err := r.instance.CopyObject(r.ctx, dstOpts, srcOpts)
	return err
}

func (r *Minio) Delete(files ...string) error {
	objectsCh := make(chan minio.ObjectInfo, len(files))
	go func() {
		defer close(objectsCh)
		for _, file := range files {
			object := minio.ObjectInfo{
				Key: file,
			}
			objectsCh <- object
		}
	}()

	for err := range r.instance.RemoveObjects(r.ctx, r.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		return err.Err
	}

	return nil
}

func (r *Minio) DeleteDirectory(directory string) error {
	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}
	opts := minio.RemoveObjectOptions{
		ForceDelete: true,
	}
	err := r.instance.RemoveObject(r.ctx, r.bucket, directory, opts)
	if err != nil {
		return err
	}

	return nil
}

func (r *Minio) Exists(file string) bool {
	_, err := r.instance.StatObject(r.ctx, r.bucket, file, minio.StatObjectOptions{})

	return err == nil
}

func (r *Minio) MakeDirectory(directory string) error {

	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}

	reader := strings.NewReader("")

	return r.Put(directory, reader, reader.Size())
}

func (r *Minio) Missing(file string) bool {
	return !r.Exists(file)
}

func (r *Minio) Move(oldFile, newFile string) error {
	if err := r.Copy(oldFile, newFile); err != nil {
		return err
	}

	return r.Delete(oldFile)
}

func (r *Minio) Path(file string) string {
	return file
}

func (r *Minio) Put(key string, file io.Reader, size int64) (err error) {

	var buffer []byte

	if buffer, err = io.ReadAll(file); err != nil {
		return err
	}

	_, err = r.instance.PutObject(
		r.ctx,
		r.bucket,
		key,
		bytes.NewReader(buffer),
		size,
		minio.PutObjectOptions{
			ContentType: http.DetectContentType(buffer),
		},
	)

	return err
}

func (r *Minio) PutFile(filePath string, source filesystem.File) (string, error) {
	return r.PutFileAs(filePath, source, lo.RandomString(40, lo.AlphanumericCharset))
}

func (r *Minio) PutFileAs(filePath string, source filesystem.File, name string) (string, error) {

	fullPath, err := util.FullPathOfFile(filePath, source, name)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(source.File())
	if err != nil {
		return "", err
	}

	reader := bytes.NewReader(data)

	if err := r.Put(fullPath, reader, reader.Size()); err != nil {
		return "", err
	}

	return fullPath, nil
}

func (r *Minio) Size(file string) (int64, error) {
	objInfo, err := r.instance.StatObject(r.ctx, r.bucket, file, minio.StatObjectOptions{})
	if err != nil {
		return 0, err
	}

	return objInfo.Size, nil
}

func (r *Minio) TemporaryUrl(file string, timer time.Duration) (string, error) {
	file = strings.TrimPrefix(file, "/")
	reqParams := make(url.Values)
	resignedURL, err := r.instance.PresignedGetObject(r.ctx, r.bucket, file, timer, reqParams)
	if err != nil {
		return "", err
	}

	return resignedURL.String(), nil
}

func (r *Minio) Url(file string) string {
	realUrl := strings.TrimSuffix(r.domain, "/")
	if !strings.HasSuffix(realUrl, r.bucket) {
		realUrl += "/" + r.bucket
	}

	return realUrl + "/" + strings.TrimPrefix(file, "/")
}
