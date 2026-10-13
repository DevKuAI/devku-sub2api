package repository

import (
	"context"
	"io"
	"mime"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type accountingStorage struct {
	config config.DesktopUpdateStorageConfig
}

func NewAccountingStorage(cfg *config.Config) accounting.Storage {
	return &accountingStorage{config: cfg.DesktopUpdateStorage}
}
func (s *accountingStorage) Ready() bool {
	return s.config.Endpoint != "" && s.config.ResourceBucket != "" && s.config.AccessKeyID != "" && s.config.SecretAccessKey != ""
}
func (s *accountingStorage) client(ctx context.Context) (*s3.Client, error) {
	if !s.Ready() {
		return nil, accounting.ErrStorage
	}
	return newS3Client(ctx, s3ClientParams{Endpoint: s.config.Endpoint, Region: s.config.Region, AccessKeyID: s.config.AccessKeyID, SecretAccessKey: s.config.SecretAccessKey, ForcePathStyle: s.config.ForcePathStyle})
}
func (s *accountingStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	client, err := s.client(ctx)
	if err != nil {
		return err
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.config.ResourceBucket, Key: &key, Body: body, ContentLength: &size, ContentType: &contentType})
	return err
}
func (s *accountingStorage) Presign(ctx context.Context, key, filename, contentType string, ttl time.Duration) (string, error) {
	client, err := s.client(ctx)
	if err != nil {
		return "", err
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	cache := "private, no-store"
	signed, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.config.ResourceBucket, Key: &key, ResponseContentDisposition: &disposition, ResponseContentType: &contentType, ResponseCacheControl: &cache}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return signed.URL, nil
}
