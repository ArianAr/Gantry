package s3

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ArianAr/Gantry/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps an S3 API client bound to a provider configuration.
type Client struct {
	S3       *s3.Client
	Provider db.Provider
}

// NewClient builds a dynamic S3 client from provider credentials.
// Supports AWS, Cloudflare R2, MinIO, Backblaze B2, Wasabi, and other S3-compatible APIs.
func NewClient(ctx context.Context, p db.Provider) (*Client, error) {
	if p.AccessKeyID == "" || p.SecretAccessKey == "" {
		return nil, fmt.Errorf("provider %q: access key and secret are required", p.Name)
	}
	region := p.Region
	if region == "" {
		region = "us-east-1"
	}

	cfg := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			p.AccessKeyID,
			p.SecretAccessKey,
			"",
		),
		HTTPClient: &http.Client{Timeout: 0}, // streaming transfers; no global timeout
	}

	var opts []func(*s3.Options)
	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint != "" {
		// Custom endpoints (R2, MinIO, B2, etc.) typically need path-style addressing.
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}

	// Prefer provider_type heuristics for path style when endpoint empty but non-AWS.
	pt := strings.ToLower(p.ProviderType)
	if endpoint == "" && (pt == "minio" || pt == "r2" || pt == "b2" || pt == "wasabi") {
		// Still require endpoint for non-AWS — fail early with a clear error.
		return nil, fmt.Errorf("provider %q (%s): custom endpoint is required", p.Name, p.ProviderType)
	}

	cli := s3.NewFromConfig(cfg, opts...)
	return &Client{S3: cli, Provider: p}, nil
}

// TestConnection lists buckets and returns latency in milliseconds.
func TestConnection(ctx context.Context, p db.Provider) (latencyMs int64, bucketCount int, err error) {
	start := time.Now()
	cli, err := NewClient(ctx, p)
	if err != nil {
		return 0, 0, err
	}
	out, err := cli.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	latencyMs = time.Since(start).Milliseconds()
	if err != nil {
		return latencyMs, 0, err
	}
	if out.Buckets != nil {
		bucketCount = len(out.Buckets)
	}
	return latencyMs, bucketCount, nil
}
