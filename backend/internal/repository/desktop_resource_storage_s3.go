package repository

import (
	"context"
	"io"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type desktopResourceStorage struct {
	config config.DesktopUpdateStorageConfig
}

func NewDesktopResourceStorage(cfg *config.Config) service.DesktopResourceStorage {
	s := &desktopResourceStorage{}
	if cfg != nil {
		s.config = cfg.DesktopUpdateStorage
	}
	return s
}

func (s *desktopResourceStorage) client(ctx context.Context) (*s3.Client, error) {
	if s.config.Endpoint == "" || s.config.ResourceBucket == "" || s.config.AccessKeyID == "" || s.config.SecretAccessKey == "" {
		return nil, service.ErrResourceStorage
	}
	return newS3Client(ctx, s3ClientParams{Endpoint: s.config.Endpoint, Region: s.config.Region, AccessKeyID: s.config.AccessKeyID, SecretAccessKey: s.config.SecretAccessKey, ForcePathStyle: s.config.ForcePathStyle})
}

func (s *desktopResourceStorage) Upload(ctx context.Context, key string, body io.Reader, size int64) error {
	client, err := s.client(ctx)
	if err != nil {
		return err
	}
	contentType := "application/zip"
	_, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.config.ResourceBucket, Key: &key, Body: body, ContentLength: &size, ContentType: &contentType})
	return err
}

func (s *desktopResourceStorage) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	client, err := s.client(ctx)
	if err != nil {
		return nil, 0, err
	}
	result, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.config.ResourceBucket, Key: &key})
	if err != nil {
		return nil, 0, err
	}
	if result.ContentLength == nil {
		_ = result.Body.Close()
		return nil, 0, service.ErrResourceStorage
	}
	return result.Body, *result.ContentLength, nil
}

func (s *desktopResourceStorage) Presign(ctx context.Context, key string, expiry time.Duration) (string, error) {
	client, err := s.client(ctx)
	if err != nil {
		return "", err
	}
	disposition, contentType, cacheControl := `attachment; filename="resource.zip"`, "application/zip", "private, no-store"
	result, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.config.ResourceBucket, Key: &key,
		ResponseContentDisposition: &disposition, ResponseContentType: &contentType, ResponseCacheControl: &cacheControl,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
