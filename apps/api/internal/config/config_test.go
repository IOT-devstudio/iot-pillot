package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Load 依赖 viper 的全局状态，每个用例前必须重置，否则用例之间会串味。
func resetViper() { viper.Reset() }

// 回归测试：db.user / db.username 的 key 拼写必须与 setDefaults 一致。
//
// 修复前 setDefaults 写的是 "db.user"，而 Load 读的是 "db.username"，
// 导致 DB.Username 恒为空字符串，pgx 回落到容器内的 OS 用户，
// 生产环境表现为 `failed to connect to user=root database=iot_pillot`。
// 环境变量 IOT_PILOT_DB_USER 传了也不生效，这是最隐蔽的部分。
func TestLoad_DBUsernameReadsDBUserKey(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}
	if cfg.DB.Username != "iot_pillot" {
		t.Errorf("DB.Username = %q，期望默认值 %q —— setDefaults 与 Load 的 key 又对不上了",
			cfg.DB.Username, "iot_pillot")
	}
}

// 环境变量必须能覆盖默认值（依赖 SetEnvKeyReplacer 把 db.user 映射到 DB_USER）。
func TestLoad_DBUserOverriddenByEnv(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")
	t.Setenv("IOT_PILOT_DB_USER", "from_env")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}
	if cfg.DB.Username != "from_env" {
		t.Errorf("DB.Username = %q，期望被环境变量覆盖为 %q", cfg.DB.Username, "from_env")
	}
}

// jwt.secret 缺失必须拒绝启动，而不是用空密钥签 HS256。
func TestLoad_RejectsEmptyJWTSecret(t *testing.T) {
	resetViper()

	_, err := Load()
	if err == nil {
		t.Fatal("jwt.secret 为空时 Load() 应当报错，实际返回 nil —— " +
			"空密钥签名的 JWT 任何人都能伪造")
	}
	if !strings.Contains(err.Error(), "jwt.secret") {
		t.Errorf("错误信息应指明是 jwt.secret 缺失，实际为: %v", err)
	}
}

// jwt.expire 单位是秒；缺省为 0 会让 exp = time.Now()，令牌签发即过期。
func TestLoad_JWTExpireHasPositiveDefault(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}
	if cfg.JWT.Expire <= 0 {
		t.Errorf("JWT.Expire = %d，必须为正整数秒，否则令牌签发即过期", cfg.JWT.Expire)
	}
}

// jwt.expire 被显式设成非正数时也要拦下。
func TestLoad_RejectsNonPositiveJWTExpire(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")
	t.Setenv("IOT_PILOT_JWT_EXPIRE", "0")

	if _, err := Load(); err == nil {
		t.Fatal("jwt.expire=0 时 Load() 应当报错")
	}
}

// 管理员**引导**名单默认为空。
//
// "默认有一个管理员"等于每个环境都带一个已知的提权入口；
// 要显式配置才会在启动时补种进 Redis。
func TestLoad_AdminUsersDefaultsToEmpty(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}
	if len(cfg.AUTH.AdminUsers) != 0 {
		t.Errorf("AdminUsers = %v，期望默认为空", cfg.AUTH.AdminUsers)
	}
}

// 逗号分隔的环境变量必须被正确切成多个用户名。
//
// 回归测试：viper.GetStringSlice 内部对字符串用 strings.Fields（按空白切分），
// 所以 "drayee,alice" 会变成 ["drayee,alice"] 这一个元素 —— 名字对不上、
// 引导静默失效（谁都不是管理员），而且不报任何错。
func TestLoad_AdminUsersSplitsCommaSeparatedEnv(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")
	t.Setenv("IOT_PILOT_AUTH_ADMIN_USERS", "drayee,alice")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}

	want := []string{"drayee", "alice"}
	if len(cfg.AUTH.AdminUsers) != len(want) {
		t.Fatalf("AdminUsers = %v，期望 %v", cfg.AUTH.AdminUsers, want)
	}
	for i, name := range want {
		if cfg.AUTH.AdminUsers[i] != name {
			t.Errorf("AdminUsers[%d] = %q，期望 %q", i, cfg.AUTH.AdminUsers[i], name)
		}
	}
}

// 空白、多余空格与重复项都要被清掉：配置里多写一个空格不该让人引导失败。
func TestLoad_AdminUsersNormalisesInput(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")
	t.Setenv("IOT_PILOT_AUTH_ADMIN_USERS", " drayee , ,drayee,  alice  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}

	want := []string{"drayee", "alice"}
	if len(cfg.AUTH.AdminUsers) != len(want) {
		t.Fatalf("AdminUsers = %v，期望去空白去重后为 %v", cfg.AUTH.AdminUsers, want)
	}
	for i, name := range want {
		if cfg.AUTH.AdminUsers[i] != name {
			t.Errorf("AdminUsers[%d] = %q，期望 %q", i, cfg.AUTH.AdminUsers[i], name)
		}
	}
}

// 只有逗号/空格时应当得到空名单，而不是 [""] 这种会意外匹配空用户名的结果。
func TestLoad_AdminUsersEmptyInputYieldsEmptyList(t *testing.T) {
	resetViper()
	t.Setenv("IOT_PILOT_JWT_SECRET", "test-secret")
	t.Setenv("IOT_PILOT_AUTH_ADMIN_USERS", " , ,, ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 意外失败: %v", err)
	}
	if len(cfg.AUTH.AdminUsers) != 0 {
		t.Errorf("AdminUsers = %v，期望为空", cfg.AUTH.AdminUsers)
	}
}
