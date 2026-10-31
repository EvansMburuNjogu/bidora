package storageminio

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"bidora/backend/internal/opportunity/application"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint, Bucket, AccessKey, SecretKey string
	RootCAs                                *x509.CertPool
	ServerName                             string
	Timeout                                time.Duration
}

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED MinIO config]") }

type Storage struct {
	client *minio.Client
	bucket string
}

func New(config Config) (*Storage, error) {
	if config.Endpoint == "" || strings.Contains(config.Endpoint, "/") || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" || config.RootCAs == nil || config.ServerName == "" || config.Timeout < time.Second || config.Timeout > 30*time.Second {
		return nil, errors.New("minio_config_invalid")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: config.ServerName, RootCAs: config.RootCAs}
	transport.DialContext = (&net.Dialer{Timeout: config.Timeout, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = config.Timeout
	transport.ResponseHeaderTimeout = config.Timeout
	client, err := minio.New(config.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""), Secure: true, Transport: transport})
	if err != nil {
		return nil, errors.New("minio_config_invalid")
	}
	return &Storage{client: client, bucket: config.Bucket}, nil
}
func (s *Storage) Verify(ctx context.Context) error {
	if ctx == nil || s == nil || s.client == nil {
		return errors.New("minio_unavailable")
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil || !exists {
		return errors.New("minio_unavailable")
	}
	cfg, err := s.client.GetBucketVersioning(ctx, s.bucket)
	if err != nil || !cfg.Enabled() {
		return errors.New("minio_versioning_required")
	}
	policy, policyErr := s.client.GetBucketPolicy(ctx, s.bucket)
	if policyErr == nil {
		if strings.TrimSpace(policy) != "" {
			return errors.New("minio_private_bucket_required")
		}
	} else if minio.ToErrorResponse(policyErr).Code != "NoSuchBucketPolicy" {
		return errors.New("minio_policy_unavailable")
	}
	return nil
}
func (s *Storage) PresignPut(ctx context.Context, key, mediaType string, size int64, sum [32]byte, ttl time.Duration) (application.UploadInstruction, error) {
	if ctx == nil || s == nil || key == "" || size < 1 || ttl <= 0 || ttl > application.UploadInstructionTTL {
		return application.UploadInstruction{}, errors.New("minio_upload_invalid")
	}
	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(s.bucket); err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	if err := policy.SetKey(key); err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	if err := policy.SetExpires(time.Now().UTC().Add(ttl)); err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	if err := policy.SetContentType(mediaType); err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	if err := policy.SetContentLengthRange(size, size); err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	if err := policy.SetChecksum(minio.NewChecksum(minio.ChecksumSHA256, sum[:])); err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	u, fields, err := s.client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return application.UploadInstruction{}, errors.New("minio_presign_failed")
	}
	return application.UploadInstruction{Method: http.MethodPost, URL: u.String(), Fields: fields}, nil
}
func (s *Storage) OpenExact(ctx context.Context, key, version string) (io.ReadCloser, error) {
	if ctx == nil || s == nil || key == "" || version == "" {
		return nil, errors.New("minio_object_invalid")
	}
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{VersionID: version})
	if err != nil {
		return nil, errors.New("minio_get_failed")
	}
	if _, err = object.Stat(); err != nil {
		_ = object.Close()
		return nil, errors.New("minio_get_failed")
	}
	return object, nil
}
func (s *Storage) Inspect(ctx context.Context, key string) (application.StoredObject, error) {
	if ctx == nil || s == nil || key == "" {
		return application.StoredObject{}, errors.New("minio_object_invalid")
	}
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{Checksum: true})
	if err != nil {
		return application.StoredObject{}, errors.New("minio_stat_failed")
	}
	if info.VersionID == "" || info.ChecksumSHA256 == "" {
		return application.StoredObject{}, errors.New("minio_object_unversioned")
	}
	raw, err := base64.StdEncoding.DecodeString(info.ChecksumSHA256)
	if err != nil || len(raw) != 32 {
		return application.StoredObject{}, errors.New("minio_checksum_invalid")
	}
	var sum [32]byte
	copy(sum[:], raw)
	return application.StoredObject{ObjectVersion: info.VersionID, MediaType: info.ContentType, SizeBytes: info.Size, SHA256: sum}, nil
}
