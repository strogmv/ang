package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

func New(ctx context.Context, region, bucket, endpoint string) (*S3Client, error) {
	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	accessKeyID := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secretAccessKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	// Local S3-compatible endpoints (MinIO) should work out-of-the-box.
	if endpoint != "" && (accessKeyID == "" || secretAccessKey == "") {
		accessKeyID = "minioadmin"
		secretAccessKey = "minioadmin"
	}
	if accessKeyID != "" && secretAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
	if endpoint != "" {
		if err := ensureBucket(ctx, client, bucket); err != nil {
			return nil, err
		}
	}

	return &S3Client{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    bucket,
	}, nil
}

func ensureBucket(ctx context.Context, client *s3.Client, bucket string) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("s3 bucket is required")
	}
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil &&
		!strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") &&
		!strings.Contains(err.Error(), "BucketAlreadyExists") {
		return fmt.Errorf("s3 ensure bucket %q: %w", bucket, err)
	}
	return nil
}

func (s *S3Client) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}
	return key, nil
}

func (s *S3Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return out.Body, nil
}

func (s *S3Client) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Client) GetURL(ctx context.Context, key string) (string, error) {
	return s.PresignGet(ctx, key, 15*time.Minute)
}

func (s *S3Client) List(ctx context.Context, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	keys := make([]string, 0, 32)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

func (s *S3Client) PresignGet(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("s3 presign get: %w", err)
	}
	return out.URL, nil
}
