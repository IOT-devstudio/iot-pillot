package utils

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/redis/rueidis"
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

func NewCodeManager(redisClient rueidis.Client, mailManager *MailManager) *CodeManager {
	return &CodeManager{redisClient: redisClient, mailManager: *mailManager}
}

func (cm *CodeManager) VerifyCode(ctx context.Context, verifier string, code string) error {
	key := fmt.Sprintf("public:verify_code:%s", verifier)
	cmd := cm.redisClient.B().Get().Key(key).Build()
	verifyCode, err := cm.redisClient.Do(ctx, cmd).ToString()
	if err != nil {
		return err
	}
	if verifyCode != code {
		return fmt.Errorf("验证码错误")
	}
	return nil
}

func (cm *CodeManager) SendVerifyCode(ctx context.Context, verifier string, verifierType VerifierType) error {
	verifyCode := fmt.Sprintf("%d", rand.Intn(1000000))
	cmd := cm.redisClient.B().Set().Key(fmt.Sprintf("public:verify_code:%s", verifier)).Value(verifyCode).Build()
	if err := cm.redisClient.Do(ctx, cmd).Error(); err != nil {
		return err
	}
	switch verifierType {
	case VerifierTypeEmail:
		return cm.mailManager.SendEmail(Email{
			Receiver: verifier,
			Subject:  "验证邮箱",
			Body:     fmt.Sprintf("您的验证码是: %s", verifyCode),
		})
	case VerifierTypePhone:
		return fmt.Errorf("手机号验证码未实现")
	default:
		return fmt.Errorf("未知的验证码类型")
	}
}
