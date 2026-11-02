package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/minio/minio-go/v7"
)

// ErrStorageUnavailable 供测试注入的存储不可用错误。
var ErrStorageUnavailable = errors.New("对象存储不可用")

// MemoryStorage 内存实现的对象存储替身。
//
// 对象存储是仓内唯一需要"外部真实服务"的依赖，单元测试不可能每次都起一个容器；
// 因此它有一个内存替身（Postgres 走的是仓库内既有的真实测试库接缝）。
//
// 它同时实现两个接口：
//   - Storage：对外的存储端口（EnsureBucket / Ping）
//   - BucketAPI：给 S3Storage 内部用的最小客户端方法集
//
// 这样 S3Storage 的幂等与并发语义、以及 HealthHandler 的探活行为，
// 都能在没有容器的情况下测。
type MemoryStorage struct {
	mu      sync.Mutex
	buckets map[string]bool
	objects map[string]memoryObject

	// ExistsErr / MakeErr 非 nil 时对应操作直接失败，用于构造故障场景。
	ExistsErr error
	MakeErr   error

	// AlwaysMissing 让 BucketExists 一律返回"不存在"。
	// 用来模拟竞态窗口：代码查的时候桶还不存在，但其实别的实例已经建好了。
	AlwaysMissing bool

	// MakeCalls 记录建桶被调用了几次（用于断言"桶已存在时不重复建"）。
	MakeCalls int
	// ExistsCalls 记录查桶被调用了几次。
	ExistsCalls int
}

// NewMemoryStorage 造一个空的存储替身。
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		buckets: map[string]bool{},
		objects: map[string]memoryObject{},
	}
}

// memoryObject 内存里存的一个对象。
type memoryObject struct {
	data        []byte
	contentType string
}

// BucketExists 返回桶是否存在。
func (m *MemoryStorage) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExistsCalls++
	if m.ExistsErr != nil {
		return false, m.ExistsErr
	}
	if m.AlwaysMissing {
		return false, nil
	}
	return m.buckets[bucketName], nil
}

// MakeBucket 建桶；桶已存在时返回"已被自己拥有"的错误码（与真实服务一致）。
//
// 返回的是 minio.ErrorResponse **值**而不是指针：minio-go 的 ToErrorResponse
// 只对 ErrorResponse 值类型做类型断言，返回指针的话它会得到一个空响应，
// 错误码取不到，"桶已存在"就会被误判成真故障。
func (m *MemoryStorage) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MakeCalls++
	if m.MakeErr != nil {
		return m.MakeErr
	}
	if m.buckets[bucketName] {
		return minio.ErrorResponse{
			Code:       codeBucketAlreadyOwnedByYou,
			Message:    "The bucket already exists and you already own it.",
			BucketName: bucketName,
			StatusCode: 409,
		}
	}
	m.buckets[bucketName] = true
	return nil
}

// EnsureBucket 实现 Storage 端口的建桶语义：桶已存在也算成功。
func (m *MemoryStorage) EnsureBucket(ctx context.Context) error {
	m.mu.Lock()
	exists := m.buckets[defaultMemoryBucket]
	m.mu.Unlock()
	if exists {
		return nil
	}
	return m.MakeBucket(ctx, defaultMemoryBucket, minio.MakeBucketOptions{})
}

// Ping 实现 Storage 端口的探活语义。
func (m *MemoryStorage) Ping(ctx context.Context) error {
	if m.ExistsErr != nil {
		return m.ExistsErr
	}
	return nil
}

// AddBucket 预置一个已存在的桶（用于"桶已存在时不该再建"的场景）。
func (m *MemoryStorage) AddBucket(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets[name] = true
}

// HasBucket 查询桶是否存在。
func (m *MemoryStorage) HasBucket(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buckets[name]
}

// PutObject 实现 Storage 端口的写入语义（流式读完整）。
func (m *MemoryStorage) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[objectKey] = memoryObject{data: data, contentType: contentType}
	return nil
}

// GetObject 实现 Storage 端口的读取语义。
func (m *MemoryStorage) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	m.mu.Lock()
	obj, ok := m.objects[objectKey]
	m.mu.Unlock()
	if !ok {
		return nil, "", 0, ErrStorageUnavailable
	}
	return io.NopCloser(bytes.NewReader(obj.data)), obj.contentType, int64(len(obj.data)), nil
}

// defaultMemoryBucket 内存替身里默认使用的桶名。
const defaultMemoryBucket = "iot-pillot"
