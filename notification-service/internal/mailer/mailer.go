package mailer

import (
	"fmt"
	"net/smtp"
)

type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewMailer(host, port, username, password, from string) *Mailer {
	return &Mailer{host: host, port: port, username: username, password: password, from: from}
}

func (m *Mailer) send(to, subject, body string) error {
	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		m.from, to, subject, body)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)
	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}

func (m *Mailer) SendConfirmation(to, repo, confirmURL string) error {
	subject := fmt.Sprintf("Confirm your subscription to %s", repo)
	body := fmt.Sprintf(
		"Hello,\n\nPlease confirm your subscription to releases of %s by clicking the link below:\n\n%s\n\nIf you did not subscribe, ignore this email.",
		repo, confirmURL,
	)
	return m.send(to, subject, body)
}

func (m *Mailer) SendReleaseNotification(to, repo, tag string) error {
	subject := fmt.Sprintf("New release: %s %s", repo, tag)
	body := fmt.Sprintf(
		"Hello,\n\nA new release of %s has been published: %s\n\nView it at: https://github.com/%s/releases/tag/%s",
		repo, tag, repo, tag,
	)
	return m.send(to, subject, body)
}
