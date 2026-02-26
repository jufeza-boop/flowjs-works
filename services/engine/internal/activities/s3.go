package activities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	fmodels "flowjs-works/engine/internal/models"
)

// S3Activity implements the `s3` node type.
//
// config fields:
//
//	bucket:        S3 bucket name (required)
//	region:        AWS region, e.g. "us-east-1" (required)
//	auth:          map — access_key_id (string), secret_access_key (string), session_token (string, optional)
//	               If auth is omitted the default AWS credential chain is used.
//	folder:        key prefix / "folder" inside the bucket
//	method:        "get" | "put" (required)
//	regex_filter:  regex to filter object keys during get
//	overwrite:     bool — overwrite existing destination objects (put only, default true)
//	local_folder:  local directory used as source (put) or destination (get)
//	files:         []interface{} of filenames to upload (put only)
type S3Activity struct{}

// Name returns the DSL type identifier for this activity.
func (a *S3Activity) Name() string { return "s3" }

// Execute runs the S3 get or put operation.
func (a *S3Activity) Execute(input map[string]interface{}, cfg map[string]interface{}, ctx *fmodels.ExecutionContext) (map[string]interface{}, error) {
	bucket, ok := cfg["bucket"].(string)
	if !ok || bucket == "" {
		return nil, fmt.Errorf("s3 activity: missing required config field 'bucket'")
	}

	region, ok := cfg["region"].(string)
	if !ok || region == "" {
		return nil, fmt.Errorf("s3 activity: missing required config field 'region'")
	}

	method, ok := cfg["method"].(string)
	if !ok || (method != "get" && method != "put") {
		return nil, fmt.Errorf("s3 activity: config field 'method' must be 'get' or 'put'")
	}

	folder, _ := cfg["folder"].(string)

	// Validate regex_filter early so callers get a clear error before any network I/O.
	if rf, ok := cfg["regex_filter"].(string); ok && rf != "" {
		if _, err := regexp.Compile(rf); err != nil {
			return nil, fmt.Errorf("s3 activity: invalid regex_filter %q: %w", rf, err)
		}
	}

	s3Client, err := buildS3Client(region, cfg)
	if err != nil {
		return nil, fmt.Errorf("s3 activity: failed to build S3 client: %w", err)
	}

	goCtx := contextFromCtx(ctx)
	switch method {
	case "get":
		return s3Get(goCtx, s3Client, bucket, folder, cfg)
	case "put":
		return s3Put(goCtx, s3Client, bucket, folder, cfg)
	default:
		return nil, fmt.Errorf("s3 activity: unknown method %q", method)
	}
}

// s3Get downloads objects from the bucket/folder to local_folder.
func s3Get(goCtx context.Context, client *s3.Client, bucket, prefix string, cfg map[string]interface{}) (map[string]interface{}, error) {
	localFolder, _ := cfg["local_folder"].(string)
	if localFolder == "" {
		localFolder = "."
	}

	// regex_filter was already validated in Execute; compile here to apply it.
	var filter *regexp.Regexp
	if rf, ok := cfg["regex_filter"].(string); ok && rf != "" {
		// Error is ignored — compilation was already validated in Execute.
		filter, _ = regexp.Compile(rf)
	}

	// List objects under prefix
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	var downloaded []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(goCtx)
		if err != nil {
			return nil, fmt.Errorf("s3 activity: failed to list objects: %w", err)
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			name := filepath.Base(key)
			if filter != nil && !filter.MatchString(name) {
				continue
			}

			resp, err := client.GetObject(goCtx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				return nil, fmt.Errorf("s3 activity: failed to get object %q: %w", key, err)
			}

			localPath := filepath.Join(localFolder, name)
			if err := writeLocalFile(localPath, resp.Body); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("s3 activity: failed to write local file %q: %w", localPath, err)
			}
			resp.Body.Close()
			downloaded = append(downloaded, name)
		}
	}

	if downloaded == nil {
		downloaded = []string{}
	}
	return map[string]interface{}{
		"files_downloaded": downloaded,
		"count":            len(downloaded),
	}, nil
}

// s3Put uploads files from config["files"] to the bucket/folder.
func s3Put(goCtx context.Context, client *s3.Client, bucket, prefix string, cfg map[string]interface{}) (map[string]interface{}, error) {
	localFolder, _ := cfg["local_folder"].(string)
	if localFolder == "" {
		localFolder = "."
	}

	overwrite := true
	if ow, ok := cfg["overwrite"].(bool); ok {
		overwrite = ow
	}

	var fileNames []string
	if flist, ok := cfg["files"].([]interface{}); ok {
		for _, f := range flist {
			if s, ok := f.(string); ok {
				fileNames = append(fileNames, s)
			}
		}
	}

	var uploaded []string
	for _, name := range fileNames {
		var key string
		if prefix == "" {
			key = name
		} else {
			key = strings.TrimRight(prefix, "/") + "/" + name
		}

		if !overwrite {
			_, err := client.HeadObject(goCtx, &s3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err == nil {
				// Object exists; skip
				continue
			}
		}

		localPath := filepath.Join(localFolder, name)
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("s3 activity: failed to read local file %q: %w", localPath, err)
		}

		_, err = client.PutObject(goCtx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(data),
		})
		if err != nil {
			return nil, fmt.Errorf("s3 activity: failed to upload %q: %w", key, err)
		}
		uploaded = append(uploaded, name)
	}

	if uploaded == nil {
		uploaded = []string{}
	}
	return map[string]interface{}{
		"files_uploaded": uploaded,
		"count":          len(uploaded),
	}, nil
}

// buildS3Client creates an AWS S3 client for the given region.
// Credentials are read from cfg["auth"] (nested map) when present, or from
// flat top-level keys (access_key_id, secret_access_key, session_token) injected
// by the secret resolver. If neither is present the default AWS credential chain
// (env vars, ~/.aws, IAM role, …) is used.
func buildS3Client(region string, cfg map[string]interface{}) (*s3.Client, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(region))

	// getCredential checks the nested auth map first, then falls back to flat config keys.
	getCredential := func(key string) string {
		if authMap, ok := cfg["auth"].(map[string]interface{}); ok {
			if v, ok := authMap[key].(string); ok {
				return v
			}
		}
		v, _ := cfg[key].(string)
		return v
	}

	accessKey := getCredential("access_key_id")
	secretKey := getCredential("secret_access_key")
	sessionToken := getCredential("session_token")
	if accessKey != "" && secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return s3.NewFromConfig(awsCfg), nil
}

// writeLocalFile writes data from r to the given path, creating the file.
func writeLocalFile(path string, r io.ReadCloser) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// contextFromCtx returns a Go context.Context for use in external API calls.
// Currently returns context.Background(); can be extended to propagate deadlines.
func contextFromCtx(_ *fmodels.ExecutionContext) context.Context {
	return context.Background()
}
