package mailer

import (
	"fmt"
	"net"
	"net/smtp"
	"time"
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
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		m.from, to, subject, body)
	addr := net.JoinHostPort(m.host, m.port)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("mailer: dial: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("mailer: set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return fmt.Errorf("mailer: new client: %w", err)
	}
	defer client.Close()

	if m.username != "" {
		auth := smtp.PlainAuth("", m.username, m.password, m.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: auth: %w", err)
		}
	}

	if err := client.Mail(m.from); err != nil {
		return fmt.Errorf("mailer: mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("mailer: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: close data: %w", err)
	}

	return client.Quit()
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
