package utils

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/redis/rueidis"
)

const (
	// VerifyCodeTTL 验证码在 Redis 里的存活时间。
	//
	// 必须有 TTL：原来的实现是裸 SET，验证码永不过期，泄露一次就等于永久可用。
	// 5 分钟足够完成一次注册，同时把泄露后的可用窗口压到最短。
	VerifyCodeTTL = 5 * time.Minute

	// VerifyCodeMaxAttempts 同一验证码允许猜错的次数，超过即作废。
	//
	// 没有上限时，6 位码（10^6 空间）可以被无限次尝试，等同于没有验证码。
	VerifyCodeMaxAttempts = 5

	// VerifyCodeDigits 验证码位数。生成时必须补齐到该位数（%0*d），
	// 否则 rand 出的小数字会变成 1~5 位，而 RegisterReq.Code 要求恰好 6 位，
	// 结果就是约 10% 的注册在接口层直接被拒、用户拿到的码永远提交不上去。
	VerifyCodeDigits = 6
)

type CodeManager struct {
	redisClient rueidis.Client
	mailManager MailManager
}

type VerifierType string

const (
	VerifierTypeEmail VerifierType = "email"
	VerifierTypePhone VerifierType = "phone"
)

// VerifyCodePurpose 验证码用途（issue #84）。
//
// 同一邮箱可能同时存在"注册"与"找回密码"两张码，key 必须带用途维度：
// 否则用户先请求注册码、再请求重置码，后者会把前者覆盖掉，
// 而注册流程却还在校验那个已经被覆盖的值。
type VerifyCodePurpose string

const (
	// VerifyPurposeRegister 注册用途。
	VerifyPurposeRegister VerifyCodePurpose = "register"
	// VerifyPurposeReset 找回密码用途。
	VerifyPurposeReset VerifyCodePurpose = "reset"
)

// 验证码用途相关的 sentinel 错误。
var (
	// ErrVerifyCodePurposeMismatch 用注册码去重置密码（或反之）。
	ErrVerifyCodePurposeMismatch = errors.New("验证码用途不匹配，请重新获取")
	// ErrVerifyCodeExpired 验证码已过期（TTL 到了）。
	ErrVerifyCodeExpired = errors.New("验证码已过期，请重新获取")
	// ErrVerifyCodeReused 验证码已被使用过（一次性）。
	ErrVerifyCodeReused = errors.New("验证码已被使用，请重新获取")
)

func NewCodeManager(redisClient rueidis.Client, mailManager *MailManager) *CodeManager {
	return &CodeManager{redisClient: redisClient, mailManager: *mailManager}
}

// GenerateVerifyCode 生成指定位数的验证码。
//
// 用 crypto/rand 而不是 math/rand：验证码是安全凭据，math/rand 的序列在知道
// 种子/输出历史后是可预测的。取值后必须补零（%0*d），否则 12345 会变成 5 位。
func GenerateVerifyCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < VerifyCodeDigits; i++ {
		max.Mul(max, big.NewInt(10))
	}

	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("生成验证码失败: %w", err)
	}

	return fmt.Sprintf("%0*d", VerifyCodeDigits, value.Int64()), nil
}

// verifyCodeKey 验证码在 Redis 中的 key，带用途维度（issue #84）。
func verifyCodeKey(verifier string, purpose VerifyCodePurpose) string {
	return fmt.Sprintf("public:verify_code:%s:%s", verifier, purpose)
}

// verifyAttemptKey 猜错次数在 Redis 中的 key，同样带用途维度：
// 注册码的猜错不该把重置码一起废掉。
func verifyAttemptKey(verifier string, purpose VerifyCodePurpose) string {
	return fmt.Sprintf("public:verify_code_attempts:%s:%s", verifier, purpose)
}

// SendVerifyCode 生成并发送验证码，同时重置该验证目标的猜错计数。
//
// 重发即重置计数是刻意的：用户"重新获取一个码"之后，之前猜错的次数不该
// 继续算在新码头上，否则正常用户会被自己的手误锁死。
func (cm *CodeManager) SendVerifyCode(ctx context.Context, verifier string, purpose VerifyCodePurpose, verifierType VerifierType) error {
	verifyCode, err := GenerateVerifyCode()
	if err != nil {
		return err
	}

	setCmd := cm.redisClient.B().Set().
		Key(verifyCodeKey(verifier, purpose)).
		Value(verifyCode).
		Ex(VerifyCodeTTL).
		Build()
	if err := cm.redisClient.Do(ctx, setCmd).Error(); err != nil {
		return err
	}

	// 计数清零失败不该阻断发送：最坏情况是沿用旧计数，用户重新获取一次即可
	_ = cm.redisClient.Do(ctx, cm.redisClient.B().Del().Key(verifyAttemptKey(verifier, purpose)).Build()).Error()

	switch verifierType {
	case VerifierTypeEmail:
		return cm.mailManager.SendEmail(Email{
			Receiver: verifier,
			Subject:  verifyEmailSubject(purpose),
			Body: fmt.Sprintf(
				"您的验证码是: %s<br>%d 分钟内有效，请勿转发给他人。若非本人操作请忽略本邮件。",
				verifyCode, int(VerifyCodeTTL.Minutes())),
		})
	case VerifierTypePhone:
		return fmt.Errorf("手机号验证码未实现")
	default:
		return fmt.Errorf("未知的验证码类型")
	}
}

// verifyEmailSubject 按用途给邮件一个可辨识的主题。
func verifyEmailSubject(purpose VerifyCodePurpose) string {
	if purpose == VerifyPurposeReset {
		return "重置密码验证码"
	}
	return "验证邮箱"
}

// ErrVerifyCodeMismatch 验证码不匹配。
var ErrVerifyCodeMismatch = errors.New("验证码错误或已过期")

// ErrVerifyCodeExhausted 猜错次数用尽，验证码已作废。
var ErrVerifyCodeExhausted = errors.New("验证码错误次数过多，请重新获取")

// verifyUsedKey 已使用标记（issue #84）。
//
// 单用 RedisNil 无法区分"过期"、"已使用"与"从未发送"，用户体验上都只说
// "验证码错误"会让人反复重试同一个码。多写一个带 TTL 的 used 标记，
// 就能把"你这个码已经用过了"和"这个码没得来/过期了"分开讲。
func verifyUsedKey(verifier string, purpose VerifyCodePurpose) string {
	return fmt.Sprintf("public:verify_code_used:%s:%s", verifier, purpose)
}

// VerifyCode 校验验证码，**成功后立即作废**。
//
// 三件事一起做，缺一不可：
//  1. 比对成功后删除该码 —— 否则同一个码可以反复用于注册，配上"用户名不唯一"
//     就能用一条验证码造出多个账号；
//  2. 猜错时累加计数，达到上限即删除该码 —— 否则 10^6 的空间可以被无限次尝试；
//  3. 码本身带 TTL（见 SendVerifyCode）。
//
// purpose 必须与 SendVerifyCode 时一致，否则会校验另一张码。
func (cm *CodeManager) VerifyCode(ctx context.Context, verifier string, purpose VerifyCodePurpose, code string) error {
	getCmd := cm.redisClient.B().Get().Key(verifyCodeKey(verifier, purpose)).Build()
	stored, err := cm.redisClient.Do(ctx, getCmd).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			// 码不存在：已过期 / 已使用 / 从未发送。用 used 标记区分后两者。
			return cm.classifyMissingCode(ctx, verifier, purpose)
		}
		return fmt.Errorf("读取验证码失败: %w", err)
	}

	if stored != code {
		return cm.recordFailedAttempt(ctx, verifier, purpose)
	}

	// 校验通过：删除验证码与计数器，并留下一个带 TTL 的使用标记
	delCmd := cm.redisClient.B().Del().Key(verifyCodeKey(verifier, purpose), verifyAttemptKey(verifier, purpose)).Build()
	if err := cm.redisClient.Do(ctx, delCmd).Error(); err != nil {
		return fmt.Errorf("作废验证码失败: %w", err)
	}
	usedCmd := cm.redisClient.B().Set().Key(verifyUsedKey(verifier, purpose)).Value("1").Ex(VerifyCodeTTL).Build()
	// 标记写失败不阻断：最坏情况是用户重复提交时拿到"过期"而不是"已使用"
	if err := cm.redisClient.Do(ctx, usedCmd).Error(); err != nil {
		log.Printf("[code] 写入验证码使用标记失败（verifier=%s purpose=%s）: %v", verifier, purpose, err)
	}

	return nil
}

// classifyMissingCode 本用途的码不存在时，判断是"用途不符 / 已使用 / 过期"。
//
// 顺序很重要：先查对侧用途的码。用户拿重置码去注册（或反之）时，
// 本用途 key 为空但对侧 key 有值 —— 此时报"验证码已过期"会误导用户
// 反复重试同一张码，而正确指引是"你拿的是另一张码"。
func (cm *CodeManager) classifyMissingCode(ctx context.Context, verifier string, purpose VerifyCodePurpose) error {
	otherPurpose := VerifyPurposeRegister
	if purpose == VerifyPurposeRegister {
		otherPurpose = VerifyPurposeReset
	}

	otherCmd := cm.redisClient.B().Exists().Key(verifyCodeKey(verifier, otherPurpose)).Build()
	otherExists, err := cm.redisClient.Do(ctx, otherCmd).ToInt64()
	if err == nil && otherExists > 0 {
		return ErrVerifyCodePurposeMismatch
	}

	// 对侧也没有码，再看本用途是否曾成功使用过
	usedCmd := cm.redisClient.B().Exists().Key(verifyUsedKey(verifier, purpose)).Build()
	used, err := cm.redisClient.Do(ctx, usedCmd).ToInt64()
	if err == nil && used > 0 {
		return ErrVerifyCodeReused
	}
	return ErrVerifyCodeExpired
}

// recordFailedAttempt 累加猜错次数，达到上限就作废验证码。
func (cm *CodeManager) recordFailedAttempt(ctx context.Context, verifier string, purpose VerifyCodePurpose) error {
	attemptKey := verifyAttemptKey(verifier, purpose)

	incrCmd := cm.redisClient.B().Incr().Key(attemptKey).Build()
	count, err := cm.redisClient.Do(ctx, incrCmd).ToInt64()
	if err != nil {
		return ErrVerifyCodeMismatch
	}

	// 只在第一次计数时设置窗口，避免每次猜错都把窗口往后推（那样永远不过期）
	if count == 1 {
		expireCmd := cm.redisClient.B().Expire().Key(attemptKey).Seconds(int64(VerifyCodeTTL.Seconds())).Build()
		_ = cm.redisClient.Do(ctx, expireCmd).Error()
	}

	if count >= VerifyCodeMaxAttempts {
		delCmd := cm.redisClient.B().Del().Key(verifyCodeKey(verifier, purpose), attemptKey).Build()
		_ = cm.redisClient.Do(ctx, delCmd).Error()
		return ErrVerifyCodeExhausted
	}

	return ErrVerifyCodeMismatch
}
