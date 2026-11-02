// Package storage 封装 S3 兼容对象存储（图片存储）。
//
// 只依赖 S3 API，不依赖任何具体发行版：换 RustFS / Garage / 云厂商 OSS
// 只需要改 endpoint 与凭据，业务代码一行不动。
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// 默认 region。
//
// 对象存储部署在单机/私有网络里时 region 基本没有意义，但 S3 协议要求它存在
// （签名与建桶都会带上）。写死一个不会与真实部署冲突的值。
const defaultRegion = "us-east-1"

// 建桶时可能回来的两个"桶已存在"错误码。
//
// 多实例同时启动会同时看到它们：两个实例都判定桶不存在、都去建，
// 后到的那个拿到 BucketAlreadyExists（别人的）或 BucketAlreadyOwnedByYou（自己的）。
// 两者都是**成功**，不是失败。
const (
	codeBucketAlreadyOwnedByYou = "BucketAlreadyOwnedByYou"
	codeBucketAlreadyExists     = "BucketAlreadyExists"
)

// 启动期等待存储就绪的重试参数。
//
// 容器编排下对象存储与应用几乎总是同时启动，谁先就绪不确定。
// 固定重试几次比"一次失败就退出"稳，但必须有上限 —— 存储真的挂了时
// 应用要尽快失败，而不是无限期挂着（容器编排会一直重启它，表现为 crash-loop 而非清晰的启动错误）。
const (
	ensureAttempts     = 5
	ensureRetryInterval = 2 * time.Second
	ensureTimeout       = 30 * time.Second
)

// Storage 对象存储的最小端口。
//
// 业务代码只依赖这个接口；测试用内存桩替换实现（见 memory.go）。
type Storage interface {
	// EnsureBucket 确保目标 bucket 存在（幂等）。
	EnsureBucket(ctx context.Context) error
	// Ping 探活存储：连不上或 bucket 不在都算不可用。
	Ping(ctx context.Context) error
}

// BucketAPI 建桶与查桶所需的最小方法集。
//
// 抽出来是为了让 EnsureBucket 的并发/幂等语义可以在不连真实对象存储的情况下测。
// *minio.Client 天然满足它。
type BucketAPI interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
}

// S3Storage 基于 S3 协议的对象存储实现。
type S3Storage struct {
	client BucketAPI
	bucket string
	region string
}

// NewS3Storage 建立对象存储客户端（不发起任何网络请求）。
func NewS3Storage(cfg *config.S3Config) (*S3Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: defaultRegion,
		// 客户端默认重试 10 次，探活与启动期建桶都会因此被拖长。
		// 这里收紧到 3 次：可重试的错误本来就该由外层的重试循环决定节奏。
		MaxRetries: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("对象存储客户端初始化失败（endpoint=%q）：%w", cfg.Endpoint, err)
	}

	return &S3Storage{client: client, bucket: cfg.Bucket, region: defaultRegion}, nil
}

// ProvideS3Storage 是给 wire 用的 provider：它吃整份配置，内部取 S3 段。
//
// 这样 config 包不需要为 storage 再加一个 provider，
// 而 storage 也不需要知道 config 里除了 S3 段以外的任何东西。
func ProvideS3Storage(cfg *config.Config) (*S3Storage, error) {
	return NewReadyS3Storage(cfg.S3)
}

// NewReadyS3Storage 建立客户端并确保 bucket 存在。
//
// 存储连不上时**直接返回错误**，让应用在启动期失败 —— 而不是启动成功、
// 等到第一次上传才炸（那时用户已经在填表单了，错误还落在最不该出的地方）。
func NewReadyS3Storage(cfg *config.S3Config) (*S3Storage, error) {
	storage, err := NewS3Storage(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ensureTimeout)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= ensureAttempts; attempt++ {
		lastErr = storage.EnsureBucket(ctx)
		if lastErr == nil {
			return storage, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("对象存储不可用（endpoint=%s, bucket=%s，重试 %d 次）：%w",
				cfg.Endpoint, cfg.Bucket, attempt, lastErr)
		case <-time.After(ensureRetryInterval):
		}
	}

	return nil, fmt.Errorf("对象存储不可用（endpoint=%s, bucket=%s，重试 %d 次）：%w",
		cfg.Endpoint, cfg.Bucket, ensureAttempts, lastErr)
}

// EnsureBucket 确保 bucket 存在。
//
// 幂等：桶已存在直接返回成功。并发安全：多实例同时启动时，
// 后到的那个拿到"已存在"错误码，同样按成功处理。
func (s *S3Storage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("查询 bucket %q 失败：%w", s.bucket, err)
	}
	if exists {
		return nil
	}

	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: s.region})
	if err == nil {
		return nil
	}

	code := minio.ToErrorResponse(err).Code
	if code == codeBucketAlreadyOwnedByYou || code == codeBucketAlreadyExists {
		return nil
	}

	// 不是"已存在"类错误：可能是与另一个实例建桶的竞态（对方刚建好、
	// 但本实例查的时候还没看到）。再查一次确认，避免把并发当成故障。
	if exists, checkErr := s.client.BucketExists(ctx, s.bucket); checkErr == nil && exists {
		return nil
	}

	return fmt.Errorf("创建 bucket %q 失败：%w", s.bucket, err)
}

// Ping 探活存储。
//
// bucket 不存在也算不可用：启动期已经建过桶，运行期桶没了说明存储被重置过，
// 这时上传必然失败，就绪探针必须报出来。
func (s *S3Storage) Ping(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("对象存储不可用：%w", err)
	}
	if !exists {
		return fmt.Errorf("对象存储 bucket %q 不存在", s.bucket)
	}
	return nil
}
