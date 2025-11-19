package collector

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3ClientFunc struct {
	GetObjectFunc      func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2Func  func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObjectCalls     int
	ListObjectsV2Calls int
}

var _ s3Client = (*s3ClientFunc)(nil)

func (c *s3ClientFunc) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	c.GetObjectCalls++
	if c.GetObjectFunc == nil {
		return nil, fmt.Errorf("GetObjectFunc is not implemented")
	}
	return c.GetObjectFunc(ctx, params, optFns...)
}

func (c *s3ClientFunc) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	c.ListObjectsV2Calls++
	if c.ListObjectsV2Func == nil {
		return nil, fmt.Errorf("ListObjectsV2Func is not implemented")
	}
	return c.ListObjectsV2Func(ctx, params, optFns...)
}
