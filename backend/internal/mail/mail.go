// Package mail sends transactional emails (login-link delivery to players).
// When SMTP is disabled the link is logged to stdout instead, so dev/test
// flows still work without an SMTP server reachable.
package mail

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

type Mailer struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Password string
	// TLS opens the connection in implicit TLS (SMTPS, typically port 465).
	// Mutually exclusive with StartTLS — pick one, not both.
	TLS bool
	// StartTLS requires the server to advertise STARTTLS and upgrades the
	// plain connection to TLS before any auth (typically port 587). If set
	// and the server doesn't offer STARTTLS, sending fails rather than
	// quietly leaking credentials.
	StartTLS   bool
	From       string
	ReplyEmail string
	ReplyName  string
	// PublicBaseURL is the origin used to build login links (no trailing slash).
	PublicBaseURL string
}

func FromEnv() *Mailer {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 25
	}
	return &Mailer{
		Enabled:       strings.EqualFold(os.Getenv("SMTP_ENABLED"), "true"),
		Host:          os.Getenv("SMTP_HOST"),
		Port:          port,
		User:          os.Getenv("SMTP_USER"),
		Password:      os.Getenv("SMTP_PASSWORD"),
		TLS:           strings.EqualFold(os.Getenv("SMTP_TLS"), "true"),
		StartTLS:      strings.EqualFold(os.Getenv("SMTP_STARTTLS"), "true"),
		From:          envOr("SMTP_FROM", "noreply@oglimmer.com"),
		ReplyEmail:    envOr("SMTP_REPLY_EMAIL", "noreply@oglimmer.com"),
		ReplyName:     envOr("SMTP_REPLY_NAME", "Trivia-Helper"),
		PublicBaseURL: strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/"),
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// LoginLinkURL builds the magic-link URL that signs the recipient back in.
// Falls back to a relative URL when PUBLIC_BASE_URL is unset, so the value is
// still useful (in logs, at minimum) during local development.
func (m *Mailer) LoginLinkURL(playerToken string) string {
	if m.PublicBaseURL == "" {
		return "/impersonate#token=" + playerToken
	}
	return m.PublicBaseURL + "/impersonate#token=" + playerToken
}

// SendLoginLink emails the recipient a one-click link that signs them back in.
// When SMTP is disabled (default in dev), the link is just logged so flows
// remain testable.
func (m *Mailer) SendLoginLink(to, playerName, gameName, gameCode, playerToken string) error {
	url := m.LoginLinkURL(playerToken)
	if !m.Enabled {
		log.Printf("[mail/disabled] login link for %q (game %q): %s", to, gameCode, url)
		return nil
	}
	if m.Host == "" {
		return errors.New("smtp host not configured")
	}
	subject := "Your Trivia login link"
	if gameName != "" {
		subject = "Your Trivia link for " + gameName
	}
	body := buildLoginBody(playerName, gameName, gameCode, url)
	return m.send(to, subject, body)
}

func buildLoginBody(playerName, gameName, gameCode, url string) string {
	greet := "Hi"
	if playerName != "" {
		greet = "Hi " + playerName
	}
	game := gameCode
	if gameName != "" {
		game = fmt.Sprintf("%s (%s)", gameName, gameCode)
	}
	return fmt.Sprintf(`%s,

You're signed up for the trivia game %s.

Open this link any time to jump back into the game:
%s

See you there!
`, greet, game, url)
}

func (m *Mailer) send(to, subject, body string) error {
	if m.TLS && m.StartTLS {
		return errors.New("smtp: TLS and STARTTLS are mutually exclusive")
	}
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	from := m.From
	if from == "" {
		from = m.ReplyEmail
	}
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	replyTo := m.ReplyEmail
	if m.ReplyName != "" {
		replyTo = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", m.ReplyName), m.ReplyEmail)
	}
	msg := strings.Builder{}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	if replyTo != "" {
		msg.WriteString("Reply-To: " + replyTo + "\r\n")
	}
	msg.WriteString("Subject: " + encodedSubject + "\r\n")
	msg.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	c, err := m.dialClient(addr)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("smtp HELO: %w", err)
	}

	if m.StartTLS {
		ok, _ := c.Extension("STARTTLS")
		if !ok {
			return errors.New("smtp: server does not advertise STARTTLS")
		}
		if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
			return fmt.Errorf("smtp STARTTLS: %w", err)
		}
	}
	// Note: with both TLS and StartTLS off, no upgrade is attempted even if the
	// server advertises STARTTLS. That keeps the flags' meaning literal — and
	// stops the upgrade path from biting users whose internal relay has a cert
	// that doesn't match the configured host (e.g. cert bound to a hostname
	// while we connect by IP). Pair "both off" with anonymous relays only,
	// since Go's PlainAuth refuses to send credentials over plaintext anyway.

	if m.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
			return fmt.Errorf("smtp AUTH: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
	}
	return c.Quit()
}

func (m *Mailer) dialClient(addr string) (*smtp.Client, error) {
	if m.TLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.Host})
		if err != nil {
			return nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		c, err := smtp.NewClient(conn, m.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp client: %w", err)
		}
		return c, nil
	}
	c, err := smtp.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("smtp dial: %w", err)
	}
	return c, nil
}
