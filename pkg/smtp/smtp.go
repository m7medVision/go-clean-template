package smtp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const (
	_defaultTimeout = 30 * time.Second
	_messageIDBytes = 16
)

// ErrInvalidAddress is returned for a From or To that is not exactly one email address.
var ErrInvalidAddress = errors.New("invalid email address")

// Client sends each message on its own connection: STARTTLS when the server offers it, then AUTH
// when credentials are set.
type Client struct {
	host    string
	addr    string
	from    string
	auth    smtp.Auth
	timeout time.Duration
}

// New returns a client for the relay at host:port that sends from from, an RFC 5322 address such as
// "App <noreply@example.com>".
func New(host string, port int, from string, opts ...Option) *Client {
	c := &Client{
		host:    host,
		addr:    net.JoinHostPort(host, strconv.Itoa(port)),
		from:    from,
		timeout: _defaultTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Send delivers m as a multipart/alternative message: the text part, then the HTML part.
func (c *Client) Send(ctx context.Context, m Message) error {
	from, err := parseAddress(c.from)
	if err != nil {
		return fmt.Errorf("smtp - Send - from: %w", err)
	}

	to, err := parseAddress(m.To)
	if err != nil {
		return fmt.Errorf("smtp - Send - to: %w", err)
	}

	data, err := build(from, to, m)
	if err != nil {
		return fmt.Errorf("smtp - Send - build: %w", err)
	}

	if err := c.deliver(ctx, from.Address, to.Address, data); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("smtp - Send: %w", ctxErr)
		}

		return fmt.Errorf("smtp - Send: %w", err)
	}

	return nil
}

// parseAddress accepts exactly one address, so a value carrying a second address or a header
// never reaches the envelope or the headers.
func parseAddress(s string) (*mail.Address, error) {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidAddress, s, err)
	}

	return addr, nil
}

func (c *Client) deliver(ctx context.Context, from, to string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dialer := net.Dialer{Timeout: c.timeout}

	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Canceling ctx closes the connection, which ends whatever command is in flight.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	if err = conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		_ = conn.Close()

		return fmt.Errorf("set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, c.host)
	if err != nil {
		_ = conn.Close()

		return fmt.Errorf("greeting: %w", err)
	}
	defer client.Close()

	return c.converse(client, from, to, data)
}

func (c *Client) converse(client *smtp.Client, from, to string, data []byte) error {
	if err := c.handshake(client); err != nil {
		return err
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}

	if err := transmit(client, data); err != nil {
		return err
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit: %w", err)
	}

	return nil
}

// handshake upgrades to TLS when the server offers it, then authenticates when credentials are set.
func (c *Client) handshake(client *smtp.Client) error {
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: c.host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if c.auth != nil {
		if err := client.Auth(c.auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	return nil
}

func transmit(client *smtp.Client, data []byte) error {
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}

	if _, err = w.Write(data); err != nil {
		return fmt.Errorf("data write: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("data close: %w", err)
	}

	return nil
}

// build writes the message: headers, then a multipart/alternative body whose parts are
// quoted-printable, which keeps every line within SMTP's limit whatever the template wrote.
func build(from, to *mail.Address, m Message) ([]byte, error) {
	id, err := messageID(from.Address)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer

	parts := multipart.NewWriter(&body)

	for _, p := range []struct{ mediaType, content string }{
		{"text/plain", m.Text},
		{"text/html", m.HTML},
	} {
		if err := writePart(parts, p.mediaType, p.content); err != nil {
			return nil, err
		}
	}

	if err := parts.Close(); err != nil {
		return nil, fmt.Errorf("multipart: %w", err)
	}

	var b bytes.Buffer

	for _, h := range [][2]string{
		{"From", from.String()},
		{"To", to.String()},
		// Q-encoding also encodes CR and LF, so a subject cannot start a new header.
		{"Subject", mime.QEncoding.Encode("utf-8", m.Subject)},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", id},
		{"MIME-Version", "1.0"},
		{"Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": parts.Boundary()})},
	} {
		b.WriteString(h[0] + ": " + h[1] + "\r\n")
	}

	b.WriteString("\r\n")
	b.Write(body.Bytes())

	return b.Bytes(), nil
}

func writePart(parts *multipart.Writer, mediaType, content string) error {
	w, err := parts.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {mediaType + "; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return fmt.Errorf("%s part: %w", mediaType, err)
	}

	qp := quotedprintable.NewWriter(w)

	if _, err = qp.Write([]byte(content)); err != nil {
		return fmt.Errorf("%s part: %w", mediaType, err)
	}

	if err = qp.Close(); err != nil {
		return fmt.Errorf("%s part: %w", mediaType, err)
	}

	return nil
}

// messageID returns a random Message-ID under the sender's domain.
func messageID(from string) (string, error) {
	b := make([]byte, _messageIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("message id: %w", err)
	}

	domain := from[strings.LastIndex(from, "@")+1:]

	return "<" + hex.EncodeToString(b) + "@" + domain + ">", nil
}
