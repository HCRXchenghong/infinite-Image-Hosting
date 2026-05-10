package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type Sender interface {
	Send(ctx context.Context, message Message) error
}

type Message struct {
	To      string
	Subject string
	Text    string
}

type ConsoleSender struct{}

func (ConsoleSender) Send(_ context.Context, message Message) error {
	fmt.Printf("[mail:dev] to=%s subject=%q body=%q\n", message.To, message.Subject, message.Text)
	return nil
}

type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

func (s SMTPSender) Send(_ context.Context, message Message) error {
	if strings.TrimSpace(s.Host) == "" {
		return ConsoleSender{}.Send(context.Background(), message)
	}
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	headers := map[string]string{
		"From":         s.From,
		"To":           message.To,
		"Subject":      message.Subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}
	var raw strings.Builder
	for key, value := range headers {
		raw.WriteString(key)
		raw.WriteString(": ")
		raw.WriteString(value)
		raw.WriteString("\r\n")
	}
	raw.WriteString("\r\n")
	raw.WriteString(message.Text)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
	if s.Username == "" && s.Password == "" {
		auth = nil
	}
	return smtp.SendMail(addr, auth, s.From, []string{message.To}, []byte(raw.String()))
}

func VerificationMessage(to, code string) Message {
	return Message{
		To:      to,
		Subject: "悦享图床邮箱验证码",
		Text:    fmt.Sprintf("您的邮箱验证码是：%s。验证码用于完成悦享图床账号验证，请勿转发给他人。", code),
	}
}

func PasswordResetMessage(to, code string) Message {
	return Message{
		To:      to,
		Subject: "悦享图床密码重置验证码",
		Text:    fmt.Sprintf("您的密码重置验证码是：%s。若非本人操作，请立即检查账号安全设置。", code),
	}
}
