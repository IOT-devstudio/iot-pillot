package utils

import (
	"fmt"

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

func (e *MailManager) SendEmail(email Email) error {
	host := e.smtp.Host
	port := e.smtp.Port
	user := e.smtp.Username
	pass := e.smtp.Password
	from := e.smtp.From
	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", email.Receiver)
	m.SetHeader("Subject", email.Subject)
	m.SetBody("text/html", email.Body)
	dialer := gomail.NewDialer(host, port, user, pass)
	if err := dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("send email failed: %w", err)
	}
	return nil
}
