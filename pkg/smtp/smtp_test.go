package smtp_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/smtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _from = "Go Clean Template <noreply@example.com>"

func welcome() smtp.Message {
	return smtp.Message{
		To:      "john@example.com",
		Subject: "Welcome, johndoe",
		HTML:    "<p>Hi johndoe,</p>\n",
		Text:    "Hi johndoe,\n",
	}
}

func TestSend_DeliversToTheEnvelopeAddresses(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	require.NoError(t, c.Send(context.Background(), welcome()))

	got := srv.only(t)
	assert.Equal(t, "noreply@example.com", got.from)
	assert.Equal(t, []string{"john@example.com"}, got.to)
}

func TestSend_WritesHeaders(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	require.NoError(t, c.Send(context.Background(), welcome()))

	msg := parse(t, srv.only(t).data)

	from, err := msg.Header.AddressList("From")
	require.NoError(t, err)
	assert.Equal(t, []*mail.Address{{Name: "Go Clean Template", Address: "noreply@example.com"}}, from)

	to, err := msg.Header.AddressList("To")
	require.NoError(t, err)
	assert.Equal(t, []*mail.Address{{Address: "john@example.com"}}, to)

	assert.Equal(t, "Welcome, johndoe", msg.Header.Get("Subject"))
	assert.Equal(t, "1.0", msg.Header.Get("MIME-Version"))
	assert.Regexp(t, `^<[0-9a-f]+@example\.com>$`, msg.Header.Get("Message-ID"))

	_, err = msg.Header.Date()
	assert.NoError(t, err)
}

func TestSend_WritesTextThenHTML(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	require.NoError(t, c.Send(context.Background(), welcome()))

	parts := alternatives(t, parse(t, srv.only(t).data))
	require.Len(t, parts, 2, "a text part, then an HTML part")
	assert.Equal(t, "text/plain", parts[0].mediaType)
	assert.Equal(t, "Hi johndoe,\n", parts[0].body)
	assert.Equal(t, "text/html", parts[1].mediaType)
	assert.Equal(t, "<p>Hi johndoe,</p>\n", parts[1].body)
}

func TestSend_EncodesANonASCIISubject(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	m := welcome()
	m.Subject = "Bienvenue, José"
	require.NoError(t, c.Send(context.Background(), m))

	raw := parse(t, srv.only(t).data).Header.Get("Subject")
	assert.NotContains(t, raw, "é", "the header itself must be ASCII")

	decoded, err := new(mime.WordDecoder).DecodeHeader(raw)
	require.NoError(t, err)
	assert.Equal(t, "Bienvenue, José", decoded)
}

func TestSend_SubjectCannotInjectHeaders(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	m := welcome()
	m.Subject = "Hi\r\nBcc: attacker@example.com"
	require.NoError(t, c.Send(context.Background(), m))

	msg := parse(t, srv.only(t).data)
	assert.Empty(t, msg.Header.Get("Bcc"))
	assert.Equal(t, []string{"john@example.com"}, srv.only(t).to)
}

func TestSend_RejectsAnInvalidRecipient(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	for _, to := range []string{"", "not an address", "john@example.com\r\nBcc: attacker@example.com"} {
		m := welcome()
		m.To = to

		err := c.Send(context.Background(), m)
		assert.ErrorIs(t, err, smtp.ErrInvalidAddress, "to %q", to)
	}

	assert.Empty(t, srv.received(), "nothing reaches the server")
}

func TestSend_RejectsAnInvalidSender(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, "not an address")

	err := c.Send(context.Background(), welcome())
	assert.ErrorIs(t, err, smtp.ErrInvalidAddress)
}

func TestSend_ReturnsTheServersRefusal(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{refuseRcpt: true})
	c := smtp.New(srv.host, srv.port, _from)

	err := c.Send(context.Background(), welcome())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "550")
	assert.Empty(t, srv.received())
}

func TestSend_AuthenticatesWhenGivenCredentials(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{auth: true})
	c := smtp.New(srv.host, srv.port, _from, smtp.Auth("user", "secret"))

	require.NoError(t, c.Send(context.Background(), welcome()))
	assert.Equal(t, "\x00user\x00secret", srv.only(t).auth)
}

func TestSend_SkipsAuthWithoutCredentials(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{auth: true})
	c := smtp.New(srv.host, srv.port, _from)

	require.NoError(t, c.Send(context.Background(), welcome()))
	assert.Empty(t, srv.only(t).auth)
}

func TestSend_StopsOnACanceledContext(t *testing.T) {
	t.Parallel()

	srv := startFakeServer(t, fakeOptions{})
	c := smtp.New(srv.host, srv.port, _from)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Send(ctx, welcome())
	assert.True(t, errors.Is(err, context.Canceled), "got %v", err)
	assert.Empty(t, srv.received())
}

// parse reads a message the way a mail client would.
func parse(t *testing.T, data string) *mail.Message {
	t.Helper()

	msg, err := mail.ReadMessage(strings.NewReader(data))
	require.NoError(t, err)

	return msg
}

type part struct {
	mediaType string
	body      string
}

// alternatives decodes the parts of a multipart/alternative message, in order.
func alternatives(t *testing.T, msg *mail.Message) []part {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/alternative", mediaType)

	var parts []part

	r := multipart.NewReader(msg.Body, params["boundary"])

	for {
		p, err := r.NextRawPart()
		if errors.Is(err, io.EOF) {
			return parts
		}

		require.NoError(t, err)

		partType, partParams, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "utf-8", partParams["charset"])
		require.Equal(t, "quoted-printable", p.Header.Get("Content-Transfer-Encoding"))

		body, err := io.ReadAll(quotedprintable.NewReader(p))
		require.NoError(t, err)

		// Lines travel as CRLF; compare them as the LF the templates wrote.
		parts = append(parts, part{mediaType: partType, body: strings.ReplaceAll(string(body), "\r\n", "\n")})
	}
}

// fakeServer is just enough of an SMTP server for net/smtp: EHLO, optional AUTH PLAIN, MAIL, RCPT,
// DATA, RSET and QUIT. It records every message it accepts.
type fakeServer struct {
	host string
	port int
	opts fakeOptions

	mu   sync.Mutex
	msgs []received
}

type fakeOptions struct {
	auth       bool // advertise AUTH PLAIN
	refuseRcpt bool // answer RCPT with 550
}

type received struct {
	auth string
	from string
	to   []string
	data string
}

func startFakeServer(t *testing.T, opts fakeOptions) *fakeServer {
	t.Helper()

	// net/smtp sends PLAIN credentials without TLS only to localhost, so the server listens there.
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ln.Close()) })

	host, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	p, err := strconv.Atoi(port)
	require.NoError(t, err)

	srv := &fakeServer{host: host, port: p, opts: opts}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go srv.serve(conn)
		}
	}()

	return srv
}

func (s *fakeServer) received() []received {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]received(nil), s.msgs...)
}

func (s *fakeServer) only(t *testing.T) received {
	t.Helper()

	msgs := s.received()
	require.Len(t, msgs, 1)

	return msgs[0]
}

// session is one connection's state. Each handler answers one command; quit ends the connection.
type session struct {
	srv  *fakeServer
	conn net.Conn
	r    *bufio.Reader
	cur  received
	quit bool
}

func (s *fakeServer) serve(conn net.Conn) {
	defer conn.Close()

	sess := &session{srv: s, conn: conn, r: bufio.NewReader(conn)}
	handlers := map[string]func(cmd string) string{
		"EHLO": sess.ehlo,
		"HELO": sess.ehlo,
		"AUTH": sess.auth,
		"MAIL": sess.mail,
		"RCPT": sess.rcpt,
		"DATA": sess.data,
		"RSET": sess.rset,
		"QUIT": sess.quitCmd,
	}

	if _, err := io.WriteString(conn, "220 fake ESMTP\r\n"); err != nil {
		return
	}

	for !sess.quit {
		line, err := sess.r.ReadString('\n')
		if err != nil {
			return
		}

		cmd := strings.TrimRight(line, "\r\n")
		verb, _, _ := strings.Cut(cmd, " ")

		reply := "502 not implemented"
		if h, ok := handlers[strings.ToUpper(verb)]; ok {
			reply = h(cmd)
		}

		if _, err := io.WriteString(conn, reply+"\r\n"); err != nil {
			return
		}
	}
}

func (s *session) ehlo(string) string {
	if s.srv.opts.auth {
		return "250-fake\r\n250 AUTH PLAIN"
	}

	return "250 fake"
}

func (s *session) auth(cmd string) string {
	fields := strings.Fields(cmd)

	decoded, err := base64.StdEncoding.DecodeString(fields[len(fields)-1])
	if err != nil {
		return "501 bad credentials"
	}

	s.cur.auth = string(decoded)

	return "235 ok"
}

func (s *session) mail(cmd string) string {
	s.cur.from = pathOf(cmd)

	return "250 ok"
}

func (s *session) rcpt(cmd string) string {
	if s.srv.opts.refuseRcpt {
		return "550 no such user"
	}

	s.cur.to = append(s.cur.to, pathOf(cmd))

	return "250 ok"
}

// data sends its own 354, since the client writes the body only after reading it, then reads the
// body up to the lone "." line, undoing dot-stuffing.
func (s *session) data(string) string {
	if _, err := io.WriteString(s.conn, "354 go ahead\r\n"); err != nil {
		return "451 write failed"
	}

	var b strings.Builder

	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return "451 read failed"
		}

		if line == ".\r\n" {
			break
		}

		b.WriteString(strings.TrimPrefix(line, "."))
	}

	s.cur.data = b.String()

	s.srv.mu.Lock()
	s.srv.msgs = append(s.srv.msgs, s.cur)
	s.srv.mu.Unlock()

	s.cur = received{auth: s.cur.auth}

	return "250 queued"
}

func (s *session) rset(string) string {
	s.cur = received{auth: s.cur.auth}

	return "250 ok"
}

func (s *session) quitCmd(string) string {
	s.quit = true

	return "221 bye"
}

// pathOf returns the address between the angle brackets of a MAIL or RCPT command.
func pathOf(cmd string) string {
	_, after, _ := strings.Cut(cmd, "<")
	path, _, _ := strings.Cut(after, ">")

	return path
}
