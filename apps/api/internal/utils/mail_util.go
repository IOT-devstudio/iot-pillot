package utils

import (
	"fmt"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/redis/rueidis"
	"gopkg.in/gomail.v2"
)

type MailManager struct {
	smtp        *config.SMTPConfig
	redisClient rueidis.Client
}

type Email struct {
	Receiver string
	Subject  string
	Body     string
}

func NewMailManager(config *config.Config, redis rueidis.Client) *MailManager {
	return &MailManager{
		smtp:        config.SMTP,
		redisClient: redis,
	}
}

// smtpDefaultTimeout 等待一次发送完成的默认上限。
const smtpDefaultTimeout = 10 * time.Second

// SendEmail 发送一封 HTML 邮件，并给整次发送加一个等待上限。
//
// 关于超时，这个库有两个必须说清楚的事实：
//
//  1. **拨号阶段**已经有 10 秒上限 —— gomail 内部写死了
//     `netDialTimeout("tcp", addr, 10*time.Second)`，Dialer 结构体上**没有**超时字段，
//     所以"给 Dialer 注入超时"这种写法在本库行不通（编译都过不了）。
//
//  2. **建连之后的 SMTP 会话没有期限**：读 banner、EHLO、AUTH、DATA，
//     以及发送最后那个点之后等待对端确认，全程没有读写 deadline。
//     对端半死不活时，DialAndSend 会一直阻塞。
//
// 因此这里用"限时等待"兜底：到点就返回错误，让调用方（HTTP 请求）能及时失败。
//
// ⚠️ 局限（刻意写出来，避免误以为已经彻底解决）：超时只是让**调用方**不再等待，
//
//	那个被卡住的 goroutine 及其 TCP 连接会一直占用到对端超时或连接被系统回收。
//	真正的彻底修复需要换成支持 context 的邮件库（见待办）。
func (e *MailManager) SendEmail(email Email) error {
	timeout := e.timeout()

	done := make(chan error, 1)
	go func() {
		done <- e.send(email)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("send email failed: 超过 %s 未收到 SMTP 响应", timeout)
	}
}

// timeout 返回等待上限：优先取配置，非法值回落到默认。
func (e *MailManager) timeout() time.Duration {
	if e.smtp == nil || e.smtp.TimeoutSeconds <= 0 {
		return smtpDefaultTimeout
	}
	return time.Duration(e.smtp.TimeoutSeconds) * time.Second
}

// send 真正的发送逻辑（在独立 goroutine 里执行，便于加等待上限）。
func (e *MailManager) send(email Email) error {
	m := gomail.NewMessage()
	m.SetHeader("From", e.smtp.From)
	m.SetHeader("To", email.Receiver)
	m.SetHeader("Subject", email.Subject)
	m.SetBody("text/html", email.Body)

	dialer := gomail.NewDialer(e.smtp.Host, e.smtp.Port, e.smtp.Username, e.smtp.Password)

	if err := dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("send email failed: %w", err)
	}
	return nil
}
