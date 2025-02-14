package service

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	log "github.com/sirupsen/logrus"
	"os"
)

type S3Service struct {
	client *s3.Client
	bucket string
}

type SimpleS3Object struct {
	Key  string `json:"key"`
	Size int64  `json:"fileSize"`
}

// MakeS3Service creates a new instance of S3Service
func MakeS3Service(region string) (*S3Service, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	return &S3Service{client: client, bucket: os.Getenv("BUCKET_NAME")}, nil
}

// ListObjects lists all objects in a bucket with given prefix
func (s *S3Service) ListObjects(prefix string) ([]SimpleS3Object, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	var objects []SimpleS3Object
	p := s3.NewListObjectsV2Paginator(s.client, input, func(o *s3.ListObjectsV2PaginatorOptions) {
		if v := int32(100); v != 0 {
			o.Limit = v
		}
	})

	// Iterate through the S3 object pages, printing each object returned.
	var i int
	for p.HasMorePages() {
		i++

		page, err := p.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("failed to get page %v, %v", i, err)
		}

		for _, obj := range page.Contents {
			// Ensures we don't get the root object which is the same as the given prefix.
			if *obj.Key != prefix {
				objects = append(objects, SimpleS3Object{
					Key:  *obj.Key,
					Size: *obj.Size,
				})
			}
		}
	}

	return objects, nil
}

// UploadObject publishes an object to S3 under the given prefix
func (s *S3Service) UploadObject(ctx context.Context, key string) (*s3.PutObjectOutput, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	result, err := s.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to put object: %v", err)
	}

	return result, nil
}

// DeleteObject deletes an object from S3
func (s *S3Service) DeleteObject(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %v", err)
	}

	return nil
}

// DeleteObjectsWithPrefix deletes all objects with the given prefix
func (s *S3Service) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	// First list all objects with the prefix
	objects, err := s.ListObjects(prefix)
	if err != nil {
		return fmt.Errorf("failed to list objects for deletion: %v", err)
	}

	// Create delete objects input
	var objectIds []types.ObjectIdentifier
	for _, obj := range objects {
		key := obj.Key
		objectIds = append(objectIds, types.ObjectIdentifier{
			Key: aws.String(key),
		})
	}

	// Delete objects in batches of 100
	const maxBatchSize = 100
	for i := 0; i < len(objectIds); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(objectIds) {
			end = len(objectIds)
		}

		batch := objectIds[i:end]
		input := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   aws.Bool(true),
			},
		}

		_, err := s.client.DeleteObjects(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to delete objects batch: %v", err)
		}
	}

	return nil
}
