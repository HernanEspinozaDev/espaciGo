package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

const smtpAddr = "127.0.0.1:11025"
const httpAddr = "127.0.0.1:11080"
const verifySubject = "EspaciGo - verifica tu correo (desarrollo)"

type mailMessage struct {
	recipient, subject, body string
	at                       time.Time
}
type mailbox struct {
	mu    sync.Mutex
	items []mailMessage
	now   func() time.Time
}

func newMailbox() *mailbox { return &mailbox{now: time.Now} }
func (b *mailbox) add(m mailMessage) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clean()
	if len(b.items) >= 20 {
		return false
	}
	m.at = b.now()
	b.items = append(b.items, m)
	return true
}
func (b *mailbox) clean() {
	cutoff := b.now().Add(-15 * time.Minute)
	out := b.items[:0]
	for _, m := range b.items {
		if m.at.After(cutoff) {
			out = append(out, m)
		}
	}
	b.items = out
}
func (b *mailbox) list() []mailMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clean()
	return append([]mailMessage(nil), b.items...)
}

type smtpServer struct {
	box     *mailbox
	ln      net.Listener
	done    chan struct{}
	mu      sync.Mutex
	clients map[net.Conn]struct{}
}

func newSMTPServer(b *mailbox) *smtpServer {
	return &smtpServer{box: b, done: make(chan struct{}), clients: make(map[net.Conn]struct{})}
}
func (s *smtpServer) Serve(ln net.Listener) error {
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	defer close(s.done)
	for {
		c, e := ln.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				continue
			}
			return e
		}
		s.mu.Lock()
		s.clients[c] = struct{}{}
		s.mu.Unlock()
		go s.client(c)
	}
}
func (s *smtpServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		_ = s.ln.Close()
	}
	for c := range s.clients {
		_ = c.Close()
	}
}
func (s *smtpServer) client(c net.Conn) {
	defer func() { _ = c.Close(); s.mu.Lock(); delete(s.clients, c); s.mu.Unlock() }()
	c.SetDeadline(time.Now().Add(30 * time.Second))
	r := bufio.NewReaderSize(c, 8192)
	w := bufio.NewWriter(c)
	reply := func(code int, text string) { fmt.Fprintf(w, "%d %s\r\n", code, text); w.Flush() }
	reply(220, "localhost local mailbox")
	var from string
	var rcpts []string
	for {
		line, e := readLineBounded(r, 8192)
		if e != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case cmd == "EHLO" || strings.HasPrefix(cmd, "EHLO "):
			fmt.Fprint(w, "250-localhost\r\n250 8BITMIME\r\n")
			w.Flush()
		case cmd == "HELO" || strings.HasPrefix(cmd, "HELO "):
			reply(250, "localhost")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			from = argAddress(line[10:])
			rcpts = nil
			if from == "" {
				reply(501, "bad sender")
			} else {
				reply(250, "ok")
			}
		case strings.HasPrefix(cmd, "RCPT TO:"):
			a := argAddress(line[8:])
			if from == "" || a == "" {
				reply(503, "bad sequence")
			} else {
				if len(rcpts) >= 10 {
					reply(452, "too many recipients")
				} else {
					rcpts = append(rcpts, a)
					reply(250, "ok")
				}
			}
		case cmd == "DATA":
			if from == "" || len(rcpts) == 0 {
				reply(503, "bad sequence")
				continue
			}
			reply(354, "end with dot")
			var data bytes.Buffer
			total := 0
			for {
				l, er := readLineBounded(r, 8192)
				if er != nil {
					return
				}
				total += len(l)
				if len(l) > 8192 || total > 65536 {
					return
				}
				if strings.TrimSuffix(strings.TrimSuffix(l, "\n"), "\r") == "." {
					break
				}
				if strings.HasPrefix(l, "..") {
					l = l[1:]
				}
				data.WriteString(l)
			}
			m, e := parseMessage(rcpts[0], data.String())
			if e != nil {
				reply(550, "invalid message")
				continue
			}
			if !s.box.add(m) {
				reply(452, "mailbox full")
			} else {
				reply(250, "queued")
			}
			from = ""
			rcpts = nil
		case cmd == "RSET":
			from = ""
			rcpts = nil
			reply(250, "reset")
		case cmd == "QUIT":
			reply(221, "bye")
			return
		default:
			reply(502, "unsupported")
		}
	}
}
func argAddress(s string) string {
	a := strings.TrimSpace(strings.TrimSuffix(s, "\r\n"))
	if strings.HasPrefix(a, "<") && strings.HasSuffix(a, ">") {
		a = a[1 : len(a)-1]
	}
	if a == "" || strings.ContainsAny(a, "\r\n\x00") {
		return ""
	}
	return a
}
func parseMessage(recipient, raw string) (mailMessage, error) {
	m, e := mail.ReadMessage(strings.NewReader(raw))
	if e != nil {
		return mailMessage{}, e
	}
	sub := m.Header.Get("Subject")
	body, e := io.ReadAll(io.LimitReader(m.Body, 65537))
	if e != nil || len(body) > 65536 {
		return mailMessage{}, errors.New("body limit")
	}
	return mailMessage{recipient: recipient, subject: sub, body: string(body)}, nil
}

var uuidRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func verificationLink(subject, body string) string {
	if subject != verifySubject {
		return ""
	}
	var ids, tokens []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "Token ID:") {
			ids = append(ids, strings.TrimSpace(strings.TrimPrefix(line, "Token ID:")))
		}
		if strings.HasPrefix(line, "Token:") {
			tokens = append(tokens, strings.TrimSpace(strings.TrimPrefix(line, "Token:")))
		}
	}
	if len(ids) != 1 || len(tokens) != 1 || !uuidRE.MatchString(ids[0]) || len(tokens[0]) == 0 || len(tokens[0]) > 2048 || strings.IndexFunc(tokens[0], func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return ""
	}
	return "https://127.0.0.1:4219/verificar-correo#token_id=" + url.QueryEscape(ids[0]) + "&token=" + url.QueryEscape(tokens[0])
}
func readLineBounded(r *bufio.Reader, max int) (string, error) {
	var b []byte
	for {
		frag, err := r.ReadSlice('\n')
		if len(b)+len(frag) > max {
			return "", errors.New("line too long")
		}
		b = append(b, frag...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func newHTTPHandler(b *mailbox) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		if r.Host != httpAddr {
			http.Error(w, "bad host", http.StatusMisdirectedRequest)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path != "/" && r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/health" {
			io.WriteString(w, "ready\n")
			return
		}
		var out strings.Builder
		out.WriteString("<!doctype html><meta charset=utf-8><title>Mailbox local</title><h1>Mensajes de verificación locales</h1>")
		for _, m := range b.list() {
			fmt.Fprintf(&out, "<article><h2>Destinatario: %s</h2><h3>%s</h3><pre>%s</pre>", html.EscapeString(m.recipient), html.EscapeString(m.subject), html.EscapeString(m.body))
			if l := verificationLink(m.subject, m.body); l != "" {
				fmt.Fprintf(&out, "<a rel=\"noreferrer\" href=\"%s\">Abrir verificación en desarrollo</a>", html.EscapeString(l))
			}
			out.WriteString("</article>")
		}
		if r.Method != "HEAD" {
			io.WriteString(w, out.String())
		}
	})
}
func runMailbox(ctx context.Context, smtpBind, httpBind string) error {
	smtpLn, err := net.Listen("tcp", smtpBind)
	if err != nil {
		return fmt.Errorf("SMTP listen: %w", err)
	}
	httpLn, err := net.Listen("tcp", httpBind)
	if err != nil {
		_ = smtpLn.Close()
		return fmt.Errorf("HTTP listen: %w", err)
	}
	b := newMailbox()
	sm := newSMTPServer(b)
	smtpErr := make(chan error, 1)
	go func() { smtpErr <- sm.Serve(smtpLn) }()
	srv := &http.Server{Handler: newHTTPHandler(b), ReadHeaderTimeout: 5 * time.Second}
	httpErr := make(chan error, 1)
	go func() { httpErr <- srv.Serve(httpLn) }()
	select {
	case <-ctx.Done():
		_ = smtpLn.Close()
		sm.Close()
		_ = srv.Close()
		<-smtpErr
		<-httpErr
		return nil
	case err := <-smtpErr:
		_ = srv.Close()
		sm.Close()
		return fmt.Errorf("SMTP serve: %w", err)
	case err := <-httpErr:
		_ = smtpLn.Close()
		sm.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP serve: %w", err)
	}
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runMailbox(ctx, smtpAddr, httpAddr); err != nil {
		fmt.Fprintln(os.Stderr, "local mailbox failed")
		os.Exit(1)
	}
}
