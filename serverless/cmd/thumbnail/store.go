package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectStore — вузький інтерфейс над S3. Завдяки йому логіку обробки
// можна тестувати без AWS і без Floci (так само, як repository у LibraTrack).
type ObjectStore interface {
	Get(ctx context.Context, bucket, key string) ([]byte, error)
	Put(ctx context.Context, bucket, key string, body []byte, contentType string) error
}

type s3Store struct{ client *s3.Client }

func newS3Store(ctx context.Context) (*s3Store, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// Усередині Floci Lambda-контейнер отримує AWS_ENDPOINT_URL.
		// Path-style адресація працює без wildcard-DNS (див. README, розділ «Мережа»).
		if os.Getenv("AWS_ENDPOINT_URL") != "" || os.Getenv("AWS_ENDPOINT_URL_S3") != "" {
			o.UsePathStyle = true
		}
	})
	return &s3Store{client: client}, nil
}

func (s *s3Store) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *s3Store) Put(ctx context.Context, bucket, key string, body []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	return err
}
