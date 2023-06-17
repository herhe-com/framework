package qiniu

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/filesystem/util"
	"github.com/herhe-com/framework/support/str"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/cdn"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/redis/go-redis/v9"
	"io"
	"os"
	"strings"
	"time"
)

/*
 * Qiniu OSS
 * Document: https://developer.qiniu.com/kodo/1277/product-introduction
 * Example: https://developer.qiniu.com/kodo/3939/overview-of-the-api
 */

type Qiniu struct {
	ctx       context.Context
	redis     *redis.Client
	manager   *storage.BucketManager
	key       string
	access    string
	secret    string
	bucket    string
	domain    string
	delimiter string
	prefix    string
}

func NewQiniu(ctx context.Context) (*Qiniu, error) {
	server := facades.Cfg.GetString("app.name")
	access := facades.Cfg.GetString("filesystem.qiniu.access")
	secret := facades.Cfg.GetString("filesystem.qiniu.secret")
	bucket := facades.Cfg.GetString("filesystem.qiniu.bucket")
	domain := facades.Cfg.GetString("filesystem.qiniu.domain")
	delimiter := facades.Cfg.GetString("filesystem.qiniu.delimiter", "/")
	prefix := facades.Cfg.GetString("filesystem.qiniu.prefix")

	q := &Qiniu{
		ctx:       ctx,
		redis:     facades.Redis,
		key:       fmt.Sprintf("%s:qiniu:token:%s", server, access),
		access:    access,
		secret:    secret,
		bucket:    bucket,
		domain:    domain,
		delimiter: delimiter,
		prefix:    strings.TrimSuffix(prefix, delimiter) + delimiter,
	}

	q.manager = storage.NewBucketManager(q.mac(), nil)

	return q, nil
}

func (r *Qiniu) Dirs(path string) (dirs []filesystem.Pathname, err error) {

	prefix := r.realPath(path)

	marker := ""

	for {

		_, prefixes, s, b, err := r.manager.ListFiles(r.bucket, prefix, r.delimiter, marker, util.MaxFileNum)
		if err != nil {
			return nil, err
		}

		for _, item := range prefixes {

			dir := filesystem.Pathname{
				Name:  strings.TrimPrefix(item, prefix),
				Path:  item,
				IsDir: true,
			}

			dirs = append(dirs, dir)
		}

		if b {
			marker = s
		} else {
			break
		}
	}

	return dirs, nil
}

func (r *Qiniu) Files(path string) (files []filesystem.Pathname, err error) {

	prefix := r.realPath(path)

	marker := ""

	for {

		entries, _, s, b, err := r.manager.ListFiles(r.bucket, prefix, r.delimiter, marker, util.MaxFileNum)
		if err != nil {
			return nil, err
		}

		for _, item := range entries {

			file := filesystem.Pathname{
				Name:  strings.TrimPrefix(item.Key, prefix),
				Path:  item.Key,
				IsDir: false,
			}

			files = append(files, file)
		}

		if b {
			marker = s
		} else {
			break
		}
	}

	return files, nil
}

func (r *Qiniu) List(path string) (list []filesystem.Pathname, err error) {

	prefix := r.realPath(path)

	dirs := make([]filesystem.Pathname, 0)
	files := make([]filesystem.Pathname, 0)

	marker := ""

	for {

		entries, prefixes, s, b, err := r.manager.ListFiles(r.bucket, prefix, r.delimiter, marker, util.MaxFileNum)
		if err != nil {
			return nil, err
		}

		for _, item := range prefixes {

			dirs = append(dirs, filesystem.Pathname{
				Name:  strings.TrimPrefix(item, prefix),
				Path:  item,
				IsDir: true,
			})
		}

		for _, item := range entries {

			files = append(files, filesystem.Pathname{
				Name:  strings.TrimPrefix(item.Key, prefix),
				Path:  item.Key,
				IsDir: false,
			})
		}

		if b {
			marker = s
		} else {
			break
		}
	}

	list = append(list, dirs...)
	list = append(list, files...)

	return list, nil
}

func (r *Qiniu) Copy(originFile, targetFile string) (err error) {

	origin := r.realPath(originFile)
	target := r.realPath(targetFile)

	return r.manager.Copy(r.bucket, origin, r.bucket, target, false)
}

func (r *Qiniu) Delete(keys ...string) (err error) {

	if len(keys) == 1 {

		return r.manager.Delete(r.bucket, r.realPath(keys[0]))
	} else if len(keys) > 1 {

		deletes := make([]string, len(keys))

		for index, item := range deletes {
			deletes[index] = storage.URIDelete(r.bucket, item)
		}

		if _, err = r.manager.Batch(deletes); err != nil {
			return err
		}
	}

	return nil
}

func (r *Qiniu) DeleteDirectory(directory string) (err error) {

	return nil
}

func (r *Qiniu) Exists(key string) bool {

	_, err := r.manager.Stat(r.bucket, r.realPath(key))

	return err == nil
}

func (r *Qiniu) MakeDirectory(directory string) error {

	return nil
}

func (r *Qiniu) Missing(file string) bool {
	return !r.Exists(file)
}

func (r *Qiniu) Move(oldFile, newFile string) error {
	return r.manager.Move(r.bucket, r.realPath(oldFile), r.bucket, r.realPath(newFile), false)
}

func (r *Qiniu) Path(file string) string {
	return file
}

func (r *Qiniu) Put(key string, file io.Reader, size int64) (err error) {

	key = r.realPath(key)

	resume := storage.NewFormUploader(nil)

	var ret storage.PutRet
	var extra storage.PutExtra

	if err = resume.Put(r.ctx, &ret, r.Token(), key, file, size, &extra); err != nil {
		return err
	}

	return nil
}

func (r *Qiniu) PutFile(filePath string, source filesystem.File) (string, error) {
	return r.PutFileAs(filePath, source, str.Random(40))
}

func (r *Qiniu) PutFileAs(filePath string, source filesystem.File, name string) (string, error) {

	fullPath, err := util.FullPathOfFile(filePath, source, name)
	if err != nil {
		return "", err
	}

	file, err := os.ReadFile(source.File())

	if err != nil {
		return "", err
	}

	var size int64 = 0

	if size, err = source.Size(); err != nil {
		return "", err
	}

	reader := bytes.NewReader(file)

	if err := r.Put(fullPath, reader, size); err != nil {
		return "", err
	}

	return fullPath, nil
}

func (r *Qiniu) Size(key string) (int64, error) {

	stat, err := r.manager.Stat(r.bucket, key)

	if err != nil {
		return 0, err
	}

	return stat.Fsize, nil
}

func (r *Qiniu) TemporaryUrl(key string, timer time.Duration) (url string, err error) {

	cryptKey := str.Random(8)

	deadline := time.Now().Add(timer).Unix()

	if url, err = cdn.CreateTimestampAntileechURL(key, cryptKey, deadline); err != nil {
		return "", err
	}

	return url, nil
}

func (r *Qiniu) WithContext(ctx context.Context) filesystem.Driver {

	driver, err := NewQiniu(ctx)

	if err != nil {
		//facades.Log.Errorf("init %s disk fail: %+v", r.disk, err)
		color.Errorf("init disk fail: %+v", err)
	}

	return driver
}

func (r *Qiniu) SetRedis(client *redis.Client) {
	r.redis = client
}

func (r *Qiniu) SetKey(key string) {
	r.key = key
}

func (r *Qiniu) Url(uri string) string {

	realUrl := strings.TrimSuffix(r.domain, "/")

	return realUrl + "/" + r.realPath(uri)
}

func (r *Qiniu) realPath(path string) (realPath string) {

	realPath = path

	if realPath == "" && r.prefix != "" {
		realPath = r.prefix
	} else if r.prefix != "" && !strings.HasPrefix(path, r.prefix) {
		realPath = r.prefix + strings.TrimPrefix(path, r.delimiter)
	}

	return realPath
}

func (r *Qiniu) mac() *qbox.Mac {
	return qbox.NewMac(r.access, r.secret)
}

func (r *Qiniu) Token() (token string) {

	var err error

	if r.redis != nil {
		token, err = r.redis.Get(r.ctx, r.key).Result()
	}

	if r.redis == nil || err == redis.Nil {

		policy := storage.PutPolicy{
			Scope:   r.bucket,
			Expires: 7200,
		}

		token = policy.UploadToken(r.mac())

		if token != "" && r.redis != nil {
			r.redis.Set(r.ctx, r.key, token, time.Duration(policy.Expires)*time.Second)
		}
	}

	return token
}
