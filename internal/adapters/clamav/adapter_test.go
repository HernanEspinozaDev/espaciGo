package clamav

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestConstructorRequiresExplicitFreshness(t *testing.T) {
	_, err := New(Config{Address: "127.0.0.1:3310", Timeout: time.Second, MaxBytes: 1024})
	if err == nil {
		t.Fatal("expected missing MaxSignatureAge to fail closed")
	}
}

func TestScanCleanInfectedAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response string
		want     dv.FileState
		wantErr  bool
	}{
		{name: "clean", response: "stream: OK", want: dv.FileClean},
		{name: "infected", response: "stream: Eicar-Test-Signature FOUND", want: dv.FileBlocked},
		{name: "malformed", response: "unexpected", want: dv.FileBlocked, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := startProtocolServer(t, tc.response)
			a, err := New(Config{
				Address: addr, Timeout: time.Second, MaxBytes: 20,
				MaxSignatureAge: 48 * time.Hour,
				Now:             func() time.Time { return time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC) },
			})
			if err != nil {
				t.Fatal(err)
			}
			state, err := a.Scan(context.Background(), []byte("synthetic"))
			if state != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("Scan() = %q, %v; want %q, error=%t", state, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestScannerUnavailableFailsClosedAsBlocked(t *testing.T) {
	a, err := New(Config{
		Address: "127.0.0.1:1", Timeout: 100 * time.Millisecond, MaxBytes: 1024,
		MaxSignatureAge: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.Scan(context.Background(), []byte("synthetic"))
	if err == nil || state != dv.FileBlocked {
		t.Fatalf("Scan() = %q, %v; want blocked with error", state, err)
	}
}

func TestScanRejectsOversizedContentAsBlocked(t *testing.T) {
	addr := startProtocolServer(t, "stream: OK")
	a, err := New(Config{
		Address: addr, Timeout: time.Second, MaxBytes: 3,
		MaxSignatureAge: 48 * time.Hour,
		Now:             func() time.Time { return time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.Scan(context.Background(), []byte("four"))
	if err == nil || state != dv.FileBlocked {
		t.Fatalf("Scan() = %q, %v; want blocked with error", state, err)
	}
}

func startProtocolServer(t *testing.T, response string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		if !readCommand(conn, 9) {
			return
		}
		_, _ = fmt.Fprintln(conn, "ClamAV 1.0 (29 Oct 2026)")
		if !readCommand(conn, 10) {
			return
		}
		for {
			var header [4]byte
			if _, err := io.ReadFull(conn, header[:]); err != nil {
				return
			}
			n := int(header[0])<<24 | int(header[1])<<16 | int(header[2])<<8 | int(header[3])
			if n == 0 {
				break
			}
			if !readCommand(conn, n) {
				return
			}
		}
		_, _ = fmt.Fprintln(conn, response)
	}()
	return listener.Addr().String()
}

func readCommand(r io.Reader, n int) bool {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return err == nil
}

func TestOptInLocalScanner(t *testing.T) {
	t.Skip("integration requires explicit local scanner fixture")
}
