package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/config"
)

// ErrInvalidRecipient is returned when an address could be used to inject headers.
var ErrInvalidRecipient = errors.New("mailer: invalid recipient")

// Mailer sends plain text e-mails.
type Mailer interface {
	Send(to, subject, body string) error
}

// SMTPMailer delivers through an SMTP relay, typically the local MTA. STARTTLS is
// used when the server offers it; authentication only when a user is configured.
type SMTPMailer struct {
	addr string
	from *mail.Address
	auth smtp.Auth
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// NewSMTPMailer validates the configuration and returns a mailer.
func NewSMTPMailer(c *config.MailConfig) (*SMTPMailer, error) {
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return nil, fmt.Errorf("mailer: invalid from address %q: %w", c.From, err)
	}
	port := c.Port
	if port == 0 {
		port = 25
	}
	m := &SMTPMailer{
		addr: net.JoinHostPort(c.Host, strconv.Itoa(port)),
		from: from,
		send: smtp.SendMail,
	}
	if c.User != "" {
		m.auth = smtp.PlainAuth("", c.User, c.Password, c.Host)
	}
	return m, nil
}

// Send builds and sends the message.
func (m *SMTPMailer) Send(to, subject, body string) error {
	msg, rcpt, err := buildMessage(m.from, to, subject, body, time.Now())
	if err != nil {
		return err
	}
	return m.send(m.addr, m.auth, m.from.Address, []string{rcpt}, msg)
}

func buildMessage(from *mail.Address, to, subject, body string, now time.Time) ([]byte, string, error) {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return nil, "", ErrInvalidRecipient
	}
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return nil, "", ErrInvalidRecipient
	}

	id := make([]byte, 12)
	rand.Read(id)
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]

	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + rcpt.String() + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + now.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + hex.EncodeToString(id) + "@" + domain + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))

	return []byte(b.String()), rcpt.Address, nil
}
