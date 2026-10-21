package cos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
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
 * Tencent COS (Cloud Object Storage)
 * Document: https://cloud.tencent.com/document/product/436
 * Compatible with S3 API
 */

type COS struct {
	ctx      context.Context
	instance *s3.Client
	access   string
	secret   string
	region   string
	bucket   string
	domain   string
}

func NewCOS(ctx context.Context, configs map[string]any) (*COS, error) {

	cfg := viper.New()

	cfg.Set("cos", configs)

	access := cfg.GetString("cos.access")
	secret := cfg.GetString("cos.secret")
	region := cfg.GetString("cos.region")
	bucket := cfg.GetString("cos.bucket")
	domain := cfg.GetString("cos.domain")
	endpoint := cfg.GetString("cos.endpoint")

	if region == "" {
		region = "ap-guangzhou"
	}

	if endpoint == "" {
		endpoint = fmt.Sprintf("https://cos.%s.myqcloud.com", region)
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
		return nil, fmt.Errorf("init cos disk error: %v", err)
	}

	return &COS{
		ctx: ctx,
		instance: s3.NewFromConfig(opt, func(options *s3.Options) {
			options.UsePathStyle = false
		}),
		access: access,
		secret: secret,
		region: region,
		bucket: bucket,
		domain: domain,
	}, nil
}

func client(ctx context.Context, access, secret, region, endpoint string) (*s3.Client, error) {

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
		return nil, fmt.Errorf("init cos disk error: %v", err)
	}

	return s3.NewFromConfig(opt, func(options *s3.Options) {
		options.UsePathStyle = false
	}), nil
}

func (r *COS) Dirs(path string) (dirs []filesystem.Pathname, err error) {

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

func (r *COS) Files(path string) (files []filesystem.Pathname, err error) {

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

func (r *COS) List(path string) (list []filesystem.Pathname, err error) {

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

func (r *COS) Copy(originFile, targetFile string) error {
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

func (r *COS) Delete(files ...string) error {

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

func (r *COS) DeleteDirectory(directory string) error {
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

func (r *COS) Exists(file string) bool {
	file = strings.TrimPrefix(file, "/")

	_, err := r.instance.GetObject(r.ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(file),
	})

	return err == nil
}

func (r *COS) MakeDirectory(directory string) error {
	directory = strings.TrimPrefix(directory, "/")

	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}

	reader := strings.NewReader("")

	return r.Put(directory, reader, reader.Size())
}

func (r *COS) Missing(file string) bool {
	return !r.Exists(file)
}

func (r *COS) Move(oldFile, newFile string) error {
	if err := r.Copy(oldFile, newFile); err != nil {
		return err
	}

	return r.Delete(oldFile)
}

func (r *COS) Path(file string) string {
	return file
}

func (r *COS) Put(key string, file io.Reader, size int64) (err error) {

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

func (r *COS) PutFile(filePath string, source filesystem.File) (string, error) {
	return r.PutFileAs(filePath, source, lo.RandomString(40, lo.AlphanumericCharset))
}

func (r *COS) PutFileAs(filePath string, source filesystem.File, name string) (string, error) {

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

func (r *COS) Size(file string) (int64, error) {
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

func (r *COS) TemporaryUrl(file string, timer time.Duration) (string, error) {
	file = strings.TrimPrefix(file, "/")

	instance, err := client(r.ctx, r.access, r.secret, r.region, r.domain)

	if err != nil {
		return "", err
	}

	presignClient := s3.NewPresignClient(instance)

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

func (r *COS) PresignedUploadUrl(file string, timer time.Duration) (string, error) {
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

// TemporaryUpload returns a temporary upload credential that allows a client
// to upload the file directly to the Tencent COS storage with a presigned PUT
// request. The credential is embedded in the Url, so the client should send
// only the raw file body, without extra headers such as Content-Type,
// otherwise the signature validation fails.
func (r *COS) TemporaryUpload(file string, timer time.Duration) (*filesystem.TemporaryUpload, error) {
	file = strings.TrimPrefix(file, "/")

	presignClient := s3.NewPresignClient(r.instance)

	request, err := presignClient.PresignPutObject(r.ctx, &s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(file),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = timer
	})

	if err != nil {
		return nil, err
	}

	return &filesystem.TemporaryUpload{
		Driver:    "cos",
		Url:       request.URL,
		Method:    request.Method,
		Headers:   signedHeaders(request.SignedHeader),
		Key:       file,
		ExpiresAt: time.Now().Add(timer),
	}, nil
}

// signedHeaders converts the signed headers of a presigned request into a
// simple map, excluding Host, which is handled by the HTTP client itself.
func signedHeaders(header http.Header) map[string]string {

	headers := make(map[string]string)

	for name, values := range header {
		if strings.EqualFold(name, "Host") || len(values) == 0 {
			continue
		}
		headers[http.CanonicalHeaderKey(name)] = values[0]
	}

	return headers
}

func (r *COS) Url(file string) string {
	realUrl := strings.TrimSuffix(r.domain, "/")
	if !strings.HasSuffix(realUrl, r.bucket) {
		realUrl += "/" + r.bucket
	}

	return realUrl + "/" + strings.TrimPrefix(file, "/")
}

func (r *COS) InitiateMultipartUpload(file string) (string, error) {

	output, err := r.instance.CreateMultipartUpload(r.ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(strings.TrimLeft(file, "/")),
	})

	if err != nil {
		return "", err
	}

	return aws.ToString(output.UploadId), nil
}

func (r *COS) UploadMultipartPart(file, uploadID string, number int64, content io.Reader, size int64) (string, error) {

	buffer, err := io.ReadAll(content)

	if err != nil {
		return "", err
	}

	output, err := r.instance.UploadPart(r.ctx, &s3.UploadPartInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(strings.TrimLeft(file, "/")),
		UploadId:      aws.String(uploadID),
		PartNumber:    aws.Int32(int32(number)),
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(size),
	})

	if err != nil {
		return "", err
	}

	return aws.ToString(output.ETag), nil
}

func (r *COS) CompleteMultipartUpload(file, uploadID string, parts []filesystem.MultipartPart) error {

	sort.Slice(parts, func(i, j int) bool { return parts[i].Number < parts[j].Number })

	completed := make([]types.CompletedPart, len(parts))

	for i, part := range parts {
		completed[i] = types.CompletedPart{
			PartNumber: aws.Int32(int32(part.Number)),
			ETag:       aws.String(part.ETag),
		}
	}

	_, err := r.instance.CompleteMultipartUpload(r.ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(r.bucket),
		Key:             aws.String(strings.TrimLeft(file, "/")),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})

	return err
}

func (r *COS) AbortMultipartUpload(file, uploadID string) error {

	_, err := r.instance.AbortMultipartUpload(r.ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(r.bucket),
		Key:      aws.String(strings.TrimLeft(file, "/")),
		UploadId: aws.String(uploadID),
	})

	return err
}
