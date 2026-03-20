package oss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/viper"

	"github.com/herhe-com/framework/contracts/filesystem"
	"github.com/herhe-com/framework/filesystem/util"
	"github.com/samber/lo"
)

/*
 * Aliyun OSS (Object Storage Service)
 * Document: https://www.alibabacloud.com/help/en/oss
 * Compatible with S3 API
 */

type OSS struct {
	ctx      context.Context
	instance *s3.Client
	bucket   string
	domain   string
}

func NewOSS(ctx context.Context, configs map[string]any) (*OSS, error) {

	cfg := viper.New()

	cfg.Set("oss", configs)

	access := cfg.GetString("oss.access")
	secret := cfg.GetString("oss.secret")
	region := cfg.GetString("oss.region")
	bucket := cfg.GetString("oss.bucket")
	domain := cfg.GetString("oss.domain")
	endpoint := cfg.GetString("oss.endpoint")

	if region == "" {
		region = "oss-cn-hangzhou"
	}

	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.aliyuncs.com", region)
	}

	opt, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithBaseEndpoint(endpoint),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: access, SecretAccessKey: secret,
			},
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("init oss disk error: %v", err)
	}

	return &OSS{
		ctx: ctx,
		instance: s3.NewFromConfig(opt, func(options *s3.Options) {
			options.UsePathStyle = false
		}),
		bucket: bucket,
		domain: domain,
	}, nil
}

func (r *OSS) Dirs(path string) (dirs []filesystem.Pathname, err error) {

	prefix := util.ValidPath(path)

	output, err := r.instance.ListObjectsV2(r.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(r.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	if err != nil {
		return nil, err
	}

	for _, object := range output.CommonPrefixes {
		dirs = append(dirs, filesystem.Pathname{
			Name:  strings.ReplaceAll(*object.Prefix, prefix, ""),
			Path:  *object.Prefix,
			IsDir: true,
		})
	}

	return dirs, nil
}

func (r *OSS) Files(path string) (files []filesystem.Pathname, err error) {

	prefix := util.ValidPath(path)

	output, err := r.instance.ListObjectsV2(r.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(r.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	if err != nil {
		return nil, err
	}

	for _, object := range output.Contents {
		files = append(files, filesystem.Pathname{
			Name:  strings.TrimPrefix(*object.Key, prefix),
			Path:  *object.Key,
			IsDir: false,
		})
	}

	return files, nil
}

func (r *OSS) List(path string) (list []filesystem.Pathname, err error) {

	prefix := util.ValidPath(path)

	output, err := r.instance.ListObjectsV2(r.ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(r.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	if err != nil {
		return nil, err
	}

	for _, object := range output.CommonPrefixes {

		list = append(list, filesystem.Pathname{
			Name:  strings.ReplaceAll(*object.Prefix, prefix, ""),
			Path:  *object.Prefix,
			IsDir: true,
		})
	}

	for _, object := range output.Contents {

		list = append(list, filesystem.Pathname{
			Name:  strings.ReplaceAll(*object.Key, prefix, ""),
			Path:  *object.Key,
			IsDir: false,
		})
	}

	return list, nil
}

func (r *OSS) Copy(originFile, targetFile string) error {
	originFile = strings.TrimPrefix(originFile, "/")
	targetFile = strings.TrimPrefix(targetFile, "/")

	_, err := r.instance.CopyObject(r.ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(r.bucket),
		Key:               aws.String(targetFile),
		CopySource:        aws.String(fmt.Sprintf("%s/%s", r.bucket, originFile)),
		MetadataDirective: types.MetadataDirectiveCopy,
	})

	return err
}

func (r *OSS) Delete(files ...string) error {

	for _, file := range files {
		file = strings.TrimPrefix(file, "/")

		_, err := r.instance.DeleteObject(r.ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(r.bucket),
			Key:    aws.String(file),
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *OSS) DeleteDirectory(directory string) error {
	directory = strings.TrimPrefix(directory, "/")

	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}

	_, err := r.instance.DeleteObject(r.ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(directory),
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *OSS) Exists(file string) bool {
	file = strings.TrimPrefix(file, "/")

	_, err := r.instance.GetObject(r.ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(file),
	})

	return err == nil
}

func (r *OSS) MakeDirectory(directory string) error {
	directory = strings.TrimPrefix(directory, "/")

	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}

	reader := strings.NewReader("")

	return r.Put(directory, reader, reader.Size())
}

func (r *OSS) Missing(file string) bool {
	return !r.Exists(file)
}

func (r *OSS) Move(oldFile, newFile string) error {
	if err := r.Copy(oldFile, newFile); err != nil {
		return err
	}

	return r.Delete(oldFile)
}

func (r *OSS) Path(file string) string {
	return file
}

func (r *OSS) Put(key string, file io.Reader, size int64) (err error) {

	var buffer []byte

	if buffer, err = io.ReadAll(file); err != nil {
		return err
	}

	_, err = r.instance.PutObject(r.ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(strings.TrimLeft(key, "/")),
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(http.DetectContentType(buffer)),
	})

	return err
}

func (r *OSS) PutFile(filePath string, source filesystem.File) (string, error) {
	return r.PutFileAs(filePath, source, lo.RandomString(40, lo.AlphanumericCharset))
}

func (r *OSS) PutFileAs(filePath string, source filesystem.File, name string) (string, error) {

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

func (r *OSS) Size(file string) (int64, error) {
	file = strings.TrimPrefix(file, "/")

	output, err := r.instance.HeadObject(r.ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(file),
	})
	if err != nil {
		return 0, err
	}

	return aws.ToInt64(output.ContentLength), nil
}

func (r *OSS) TemporaryUrl(file string, timer time.Duration) (string, error) {
	file = strings.TrimPrefix(file, "/")

	presignClient := s3.NewPresignClient(r.instance)

	request, err := presignClient.PresignGetObject(r.ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(file),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = timer
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (r *OSS) PresignedUploadUrl(file string, timer time.Duration) (string, error) {
	file = strings.TrimPrefix(file, "/")

	presignClient := s3.NewPresignClient(r.instance)

	request, err := presignClient.PresignPutObject(r.ctx, &s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(file),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = timer
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (r *OSS) Url(file string) string {
	realUrl := strings.TrimSuffix(r.domain, "/")
	if !strings.HasSuffix(realUrl, r.bucket) {
		realUrl += "/" + r.bucket
	}

	return realUrl + "/" + strings.TrimPrefix(file, "/")
}
