package repository

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type S3DesktopUpdateArtifactStorage struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

var _ service.DesktopUpdateArtifactStorage = (*S3DesktopUpdateArtifactStorage)(nil)

func NewS3DesktopUpdateArtifactStorage(ctx context.Context, cfg *config.DesktopUpdateStorageConfig) (*S3DesktopUpdateArtifactStorage, error) {
	client, err := newS3Client(ctx, s3ClientParams{
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		ForcePathStyle:  cfg.ForcePathStyle,
	})
	if err != nil {
		return nil, err
	}
	return &S3DesktopUpdateArtifactStorage{
		client: client, bucket: cfg.Bucket, publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

func (s *S3DesktopUpdateArtifactStorage) Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          body,
		ContentLength: &size,
		ContentType:   &contentType,
	})
	finish()
	if err != nil {
		return "", fmt.Errorf("S3 PutObject desktop update artifact: %w", err)
	}
	return s.publicBaseURL + "/" + strings.TrimLeft(key, "/"), nil
}
