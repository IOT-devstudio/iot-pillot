// Package storage 封装 S3 兼容对象存储（图片存储）。
//
// 只依赖 S3 API，不依赖任何具体发行版：换 RustFS / Garage / 云厂商 OSS
// 只需要改 endpoint 与凭据，业务代码一行不动。
package storage

import (
	"context"
	"fmt"
	"io"
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
	ensureAttempts      = 5
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
	// PutObject 写入一个对象（流式：不落临时文件、不整块进内存）。
	PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	// GetObject 读取一个对象；返回的 ReadCloser 由调用方关闭。
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error)
	// DeleteObject 删除一个对象（孤儿清理时连同数据库行一起删）。
	DeleteObject(ctx context.Context, objectKey string) error
}

// BucketAPI 建桶与查桶所需的最小方法集。
//
// 抽出来是为了让 EnsureBucket 的并发/幂等语义可以在不连真实对象存储的情况下测。
// *minio.Client 天然满足它。
type BucketAPI interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
}

// ObjectAPI 读写对象所需的最小方法集。
//
// 同样从 *minio.Client 上抽：业务代码只依赖 Storage 端口，
// 真实客户端的具体类型（含它的多重重试、分片逻辑）不外泄。
type ObjectAPI interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error)
	// DeleteObject 删除对象；对象不存在不算错误（幂等删除，孤儿清理可重试）。
	DeleteObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

// S3Storage 基于 S3 协议的对象存储实现。
//
// client 只负责建桶/探活，objects 负责对象读写：两者是同一台 *minio.Client
// 的不同方法集，拆开是为了让「桶不存在时不该建桶」这类语义可以在没有
// 对象存储的情况下测（client 用内存替身），而对象读写走另一个适配层。
type S3Storage struct {
	client  BucketAPI
	objects ObjectAPI
	bucket  string
	region  string
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

	return &S3Storage{
		client:  client,
		objects: objectAPIClient{client},
		bucket:  cfg.Bucket,
		region:  defaultRegion,
	}, nil
}

// objectAPIClient 把 *minio.Client 适配成 ObjectAPI。
//
// *minio.Client 的 GetObject 返回 *minio.Object（带自己的 Stat/Close），
// 而 ObjectAPI 要的是「直接能读的流 + 元信息」——这里在适配层把 Stat 提前做掉，
// 让上层不用知道 minio 的对象模型。
type objectAPIClient struct {
	*minio.Client
}

func (c objectAPIClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, minio.ObjectInfo, error) {
	obj, err := c.Client.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}
	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, minio.ObjectInfo{}, err
	}
	return obj, stat, nil
}

// DeleteObject 适配 *minio.Client 的删除（幂等：对象不存在也成功）。
func (c objectAPIClient) DeleteObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return c.Client.RemoveObject(ctx, bucketName, objectName, opts)
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

// PutObject 写入一个对象（流式，不落临时文件、不整块进内存）。
//
// contentType 必须是**按真实字节嗅探**出来的类型，而不是客户端声明的：
// 声明与真实不符时以真实为准（见 service 的 UploadAttachment）。
func (s *S3Storage) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := s.objects.PutObject(ctx, s.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// GetObject 读取一个对象。
//
// 返回的 io.ReadCloser 由调用方负责关闭（通常是 defer Close）。
// contentType 与 size 取自对象元信息，不额外请求一次。
func (s *S3Storage) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	rc, info, err := s.objects.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", 0, err
	}
	return rc, info.ContentType, info.Size, nil
}

// DeleteObject 删除一个对象（孤儿清理用）。
//
// 对象不存在时返回 nil：清理任务要能幂等重试，孤儿行被并发删掉一条不该让
// 下一轮崩在 NotFound 上。
func (s *S3Storage) DeleteObject(ctx context.Context, objectKey string) error {
	return s.objects.DeleteObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}
