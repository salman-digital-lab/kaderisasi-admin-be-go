package storage

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"kaderisasi/admin/internal/config"
	"net/url"
	"strings"
)

type Store interface {
	Put(context.Context, string, []byte, string) error
	Copy(context.Context, string, string) error
	Delete(context.Context, string) error
	Get(context.Context, string) ([]byte, error)
}
type S3 struct {
	Client      *s3.Client
	Bucket      string
	Created     func(string) error
	SupportsACL bool
}

func New(c config.Config) *S3 {
	client := s3.New(s3.Options{Region: c.DriveRegion, Credentials: credentials.NewStaticCredentialsProvider(c.DriveKey, c.DriveSecret, ""), BaseEndpoint: aws.String(c.DriveEndpoint), UsePathStyle: c.DriveDisk == "minio"})
	return &S3{Client: client, Bucket: c.DriveBucket, SupportsACL: c.DriveDisk == "minio"}
}
func validKey(key string) bool {
	return key != "" && !strings.HasPrefix(key, "/") && !strings.Contains(key, "\\") && !strings.Contains(key, "://") && !strings.Contains("/"+key+"/", "/../") && !strings.Contains("/"+key+"/", "/./")
}
func (s *S3) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if !validKey(key) {
		return errors.New("invalid storage key")
	}
	if s.Created != nil {
		if err := s.Created(key); err != nil {
			return err
		}
	}
	input := &s3.PutObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String(contentType), CacheControl: aws.String("public, max-age=31536000, immutable")}
	if s.SupportsACL {
		input.ACL = types.ObjectCannedACLPublicRead
	}
	_, err := s.Client.PutObject(ctx, input)
	return err
}
func (s *S3) Copy(ctx context.Context, source, target string) error {
	if !validKey(source) || !validKey(target) {
		return errors.New("invalid storage key")
	}
	if s.Created != nil {
		if err := s.Created(target); err != nil {
			return err
		}
	}
	input := &s3.CopyObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(target), CopySource: aws.String(url.PathEscape(s.Bucket + "/" + source))}
	if s.SupportsACL {
		acl, err := s.Client.GetObjectAcl(ctx, &s3.GetObjectAclInput{Bucket: aws.String(s.Bucket), Key: aws.String(source)})
		if err != nil {
			return err
		}
		input.ACL = types.ObjectCannedACLPrivate
		for _, grant := range acl.Grants {
			if grant.Grantee != nil && aws.ToString(grant.Grantee.URI) == "http://acs.amazonaws.com/groups/global/AllUsers" && (grant.Permission == types.PermissionRead || grant.Permission == types.PermissionFullControl) {
				input.ACL = types.ObjectCannedACLPublicRead
			}
		}
	}
	_, err := s.Client.CopyObject(ctx, input)
	return err
}
func (s *S3) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return errors.New("invalid storage key")
	}
	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	return err
}
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	if !validKey(key) {
		return nil, errors.New("invalid storage key")
	}
	out, err := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(io.LimitReader(out.Body, 32<<20))
}
