package config

import (
	"os"
	"strings"
	"testing"
)

// TestValidate_S3EndpointRejectsScheme endpoint 带 scheme 必须被拒绝。
//
// endpoint 的协议由 s3.secure 单独决定；两处都写协议会出现
// "http://s3:9000" 这种没法解析的地址，而且没人说得清哪个说了算。
func TestValidate_S3EndpointRejectsScheme(t *testing.T) {
	cfg := &Config{
		JWT: &JWTConfig{Secret: "x", Expire: 3600},
		S3:  &S3Config{Endpoint: "http://s3:9000", Bucket: "iot-pillot"},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("endpoint 带 scheme 时必须报错")
	}
	if !strings.Contains(err.Error(), "不得带 scheme") {
		t.Fatalf("错误信息应说明不得带 scheme，实际: %v", err)
	}
}

// TestValidate_S3EndpointRequired endpoint 缺失必须报错。
//
// 对象存储是启动期硬依赖：应用启动时就要连上去建桶。
// 缺配置时静默用默认值，容器会以"连不上"的形式失败，难定位。
func TestValidate_S3EndpointRequired(t *testing.T) {
	cfg := &Config{
		JWT: &JWTConfig{Secret: "x", Expire: 3600},
		S3:  &S3Config{Bucket: "iot-pillot"},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("endpoint 缺失时必须报错")
	}
	if !strings.Contains(err.Error(), "s3.endpoint") {
		t.Fatalf("错误信息应点名 s3.endpoint，实际: %v", err)
	}
}

// TestValidate_S3BucketRequired bucket 缺失必须报错。
func TestValidate_S3BucketRequired(t *testing.T) {
	cfg := &Config{
		JWT: &JWTConfig{Secret: "x", Expire: 3600},
		S3:  &S3Config{Endpoint: "s3:9000"},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("bucket 缺失时必须报错")
	}
	if !strings.Contains(err.Error(), "s3.bucket") {
		t.Fatalf("错误信息应点名 s3.bucket，实际: %v", err)
	}
}

// TestLoad_S3FromEnv 环境变量前缀与替换规则要能落到 S3 段。
func TestLoad_S3FromEnv(t *testing.T) {
	t.Setenv("IOT_PILOT_S3_ENDPOINT", "s3:9000")
	t.Setenv("IOT_PILOT_S3_SECURE", "true")
	t.Setenv("IOT_PILOT_S3_BUCKET", "my-bucket")
	t.Setenv("IOT_PILOT_S3_ACCESS_KEY", "ak")
	t.Setenv("IOT_PILOT_S3_SECRET_KEY", "sk")
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")
	os.Unsetenv("IOT_PILOT_DB_HOST")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.S3 == nil {
		t.Fatal("S3 配置段不应为空")
	}
	if cfg.S3.Endpoint != "s3:9000" || !cfg.S3.Secure || cfg.S3.Bucket != "my-bucket" {
		t.Fatalf("S3 配置读取不对: %+v", cfg.S3)
	}
	if cfg.S3.AccessKey != "ak" || cfg.S3.SecretKey != "sk" {
		t.Fatalf("S3 凭据读取不对: %+v", cfg.S3)
	}
}
