package main

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestVerificationLinkRequiresExactSingleTokenIDLine(t *testing.T) {
	body := "Reference 123e4567-e89b-12d3-a456-426614174000\nToken ID: 223e4567-e89b-82d3-a456-426614174000\nToken: secret\n"
	if got := verificationLink(verifySubject, body); !strings.Contains(got, "token_id=223e4567-e89b-82d3-a456-426614174000") {
		t.Fatalf("link did not use exact Token ID line: %q", got)
	}
	for _, bad := range []string{
		"Token ID: 123e4567-e89b-12d3-a456-426614174000\nToken ID: 223e4567-e89b-82d3-a456-426614174000\nToken: x\n",
		"123e4567-e89b-12d3-a456-426614174000\nToken: x\n",
		"Token ID: 123e4567-e89b-12d3-a456-426614174000\nToken: " + strings.Repeat("x", 2049) + "\n",
		"Token ID: 123e4567-e89b-12d3-a456-426614174000\nToken: x\x01y\n",
	} {
		if got := verificationLink(verifySubject, bad); got != "" {
			t.Fatalf("accepted invalid token body: %q", got)
		}
	}
}

func TestRunMailboxContextAndBindErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runMailbox(ctx, "127.0.0.1:0", "127.0.0.1:0"); err != nil {
		t.Fatalf("canceled startup: %v", err)
	}
	want := errors.New("bind")
	if err := runMailbox(ctx, "bad address", "127.0.0.1:0"); err == nil || errors.Is(err, want) {
		t.Fatalf("expected bind error, got %v", err)
	}
}

func TestSMTPToLocalUIVerificationLink(t *testing.T) {
	box := newMailbox()
	srv := newSMTPServer(box)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer srv.Close()
	msg := "From: dev@example.test\r\nTo: user@example.test\r\nSubject: EspaciGo - verifica tu correo (desarrollo)\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nToken ID: 123e4567-e89b-12d3-a456-426614174000\nToken: abc&<secret>\n.\n"
	if err := smtp.SendMail(ln.Addr().String(), nil, "dev@example.test", []string{"user@example.test"}, []byte(msg)); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1:11080/", nil)
	r.Host = "127.0.0.1:11080"
	w := httptest.NewRecorder()
	newHTTPHandler(box).ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "https://127.0.0.1:4219/verificar-correo#token_id=123e4567-e89b-12d3-a456-426614174000&amp;token=abc%26%3Csecret%3E") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "abc&<secret>") {
		t.Fatal("unescaped token")
	}
}

func TestHostAndMethodDenied(t *testing.T) {
	h := newHTTPHandler(newMailbox())
	for _, tc := range []struct{ method, host string }{{"GET", "attacker.test"}, {"POST", "127.0.0.1:11080"}, {"PUT", "127.0.0.1:11080"}} {
		r := httptest.NewRequest(tc.method, "http://127.0.0.1:11080/", nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatalf("%s %s accepted: %d", tc.method, tc.host, w.Code)
		}
	}
}

func TestMailboxCapacityAndExpiry(t *testing.T) {
	b := newMailbox()
	b.now = func() time.Time { return time.Unix(100, 0) }
	for i := 0; i < 20; i++ {
		if !b.add(mailMessage{recipient: "u", body: "b"}) {
			t.Fatal("rejected before cap")
		}
	}
	if b.add(mailMessage{}) {
		t.Fatal("accepted over cap")
	}
	b.now = func() time.Time { return time.Unix(100+16*60, 0) }
	if got := len(b.list()); got != 0 {
		t.Fatalf("expired count=%d", got)
	}
}
