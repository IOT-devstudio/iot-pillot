package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
)

// newTestStorage 造一个用内存替身驱动的 S3Storage。
func newTestStorage(client BucketAPI, bucket string) *S3Storage {
	return &S3Storage{client: client, bucket: bucket, region: defaultRegion}
}

// TestEnsureBucket_CreatesWhenMissing 桶不存在时要建出来。
func TestEnsureBucket_CreatesWhenMissing(t *testing.T) {
	client := NewMemoryStorage()
	s := newTestStorage(client, "iot-pillot")

	if err := s.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("建桶失败: %v", err)
	}
	if !client.HasBucket("iot-pillot") {
		t.Fatal("桶未被创建")
	}
	if client.MakeCalls != 1 {
		t.Fatalf("建桶应调用 1 次，实际 %d 次", client.MakeCalls)
	}
}

// TestEnsureBucket_Idempotent 桶已存在时既不报错也不重复建。
//
// 幂等是硬要求：应用每次重启都会调它一遍，重复建桶会让启动失败。
func TestEnsureBucket_Idempotent(t *testing.T) {
	client := NewMemoryStorage()
	client.AddBucket("iot-pillot")
	s := newTestStorage(client, "iot-pillot")

	if err := s.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("桶已存在时不应报错: %v", err)
	}
	if client.MakeCalls != 0 {
		t.Fatalf("桶已存在时不应再建桶，实际调用 %d 次", client.MakeCalls)
	}
}

// TestEnsureBucket_ConcurrentCreationIsSuccess 模拟多实例同时建桶：
// 后到的那个拿到"已存在"错误码，必须按成功处理。
//
// 这是真实场景：两个副本同时启动、都判定桶不存在、都去建，
// 后到的那个拿到 409 —— 那是**成功**，不是故障。
func TestEnsureBucket_ConcurrentCreationIsSuccess(t *testing.T) {
	client := NewMemoryStorage()
	// 查桶时一律说"不存在"：模拟竞态窗口 —— 本实例查的时候桶还没出现，
	// 其实另一个实例已经建好了。
	client.AlwaysMissing = true
	// 建桶返回"已存在"：另一个实例抢先建成功的结果。
	client.MakeErr = minio.ErrorResponse{
		Code:       codeBucketAlreadyOwnedByYou,
		Message:    "The bucket already exists and you already own it.",
		BucketName: "iot-pillot",
		StatusCode: 409,
	}
	s := newTestStorage(client, "iot-pillot")

	if err := s.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("并发建桶的已存在错误必须按成功处理: %v", err)
	}
}

// TestEnsureBucket_PropagatesRealFailure 真故障必须报错，不能被吞掉。
func TestEnsureBucket_PropagatesRealFailure(t *testing.T) {
	client := NewMemoryStorage()
	client.MakeErr = errors.New("权限不足")
	s := newTestStorage(client, "iot-pillot")

	err := s.EnsureBucket(context.Background())
	if err == nil {
		t.Fatal("建桶真故障时必须返回错误")
	}
}

// TestEnsureBucket_QueryFailurePropagates 查桶失败必须报错。
func TestEnsureBucket_QueryFailurePropagates(t *testing.T) {
	client := NewMemoryStorage()
	client.ExistsErr = errors.New("连接被拒")
	s := newTestStorage(client, "iot-pillot")

	err := s.EnsureBucket(context.Background())
	if err == nil {
		t.Fatal("查桶失败时必须返回错误")
	}
}

// TestPing_BucketMissingIsUnhealthy 桶不在时探活必须失败。
//
// 运行期桶没了说明存储被重置过，这时上传必然失败，
// 就绪探针必须报出来而不是继续返回 ready。
func TestPing_BucketMissingIsUnhealthy(t *testing.T) {
	client := NewMemoryStorage()
	s := newTestStorage(client, "iot-pillot")

	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("桶不存在时探活必须失败")
	}

	client.AddBucket("iot-pillot")
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("桶存在时探活应成功: %v", err)
	}
}

// TestPing_UnreachableIsUnhealthy 连不上时探活必须失败。
func TestPing_UnreachableIsUnhealthy(t *testing.T) {
	client := NewMemoryStorage()
	client.ExistsErr = errors.New("连接被拒")
	s := newTestStorage(client, "iot-pillot")

	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("存储不可达时探活必须失败")
	}
}
