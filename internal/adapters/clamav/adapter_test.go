package clamav

import (
	"context"
	"encoding/binary"
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
			result, err := a.ScanDetailed(context.Background(), []byte("synthetic"))
			if result.State != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("ScanDetailed() = %+v, %v; want state %q, error=%t", result, err, tc.want, tc.wantErr)
			}
			if !tc.wantErr && (result.ScannerVersion != "ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026" || !result.SignaturesAt.Equal(time.Date(2026, 10, 28, 6, 24, 17, 0, time.UTC))) {
				t.Fatalf("scan metadata missing or incorrect: %+v", result)
			}
		})
	}
}

func TestValidateVersionUsesClamDSignatureTimestamp(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	if err := validateVersion("ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026", now, 48*time.Hour); err != nil {
		t.Fatalf("fresh ClamD version rejected: %v", err)
	}
	if err := validateVersion("ClamAV 1.5.4/28100/Tue Oct 20 06:24:17 2026", now, 48*time.Hour); err == nil {
		t.Fatal("stale ClamD signature timestamp accepted")
	}
}

func TestValidateScanResultMetadata(t *testing.T) {
	valid := ScanResult{
		State:          dv.FileBlocked,
		ScannerVersion: "ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026",
		SignaturesAt:   time.Date(2026, 10, 28, 6, 24, 17, 0, time.UTC),
	}
	for _, tc := range []struct {
		name   string
		result ScanResult
		valid  bool
	}{
		{name: "valid infected result", result: valid, valid: true},
		{name: "malformed version", result: ScanResult{State: valid.State, ScannerVersion: valid.ScannerVersion + "\nforged", SignaturesAt: valid.SignaturesAt}},
		{name: "mismatched timestamp", result: ScanResult{State: valid.State, ScannerVersion: valid.ScannerVersion, SignaturesAt: valid.SignaturesAt.Add(time.Second)}},
		{name: "unsupported state", result: ScanResult{State: dv.FileError, ScannerVersion: valid.ScannerVersion, SignaturesAt: valid.SignaturesAt}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateScanResultMetadata(tc.result)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateScanResultMetadata() error=%v, valid=%t", err, tc.valid)
			}
		})
	}
}

func TestStaleSignaturesFailClosedAsBlocked(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if readCommand(conn, 9) {
			_, _ = fmt.Fprint(conn, "ClamAV 1.5.4/28100/Tue Oct 20 06:24:17 2026", string([]byte{0}))
		}
	}()
	a, err := New(Config{Address: listener.Addr().String(), Timeout: time.Second, MaxBytes: 1024, MaxSignatureAge: 48 * time.Hour, Now: func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ScanDetailed(context.Background(), []byte("synthetic"))
	if err == nil || result.State != dv.FileBlocked || result.ScannerVersion != "" || !result.SignaturesAt.IsZero() {
		t.Fatalf("ScanDetailed() = %+v, %v; want blocked with no metadata on stale signatures", result, err)
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
	result, err := a.ScanDetailed(context.Background(), []byte("synthetic"))
	if err == nil || result.State != dv.FileBlocked || result.ScannerVersion != "" || !result.SignaturesAt.IsZero() {
		t.Fatalf("ScanDetailed() = %+v, %v; want blocked with no metadata when unavailable", result, err)
	}
}

func TestScanVersionTimeoutFailsClosed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = readCommand(conn, 9)
		time.Sleep(150 * time.Millisecond)
	}()
	a, err := New(Config{Address: listener.Addr().String(), Timeout: 25 * time.Millisecond, MaxBytes: 1024, MaxSignatureAge: 48 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.Scan(context.Background(), []byte("synthetic"))
	if err == nil || state != dv.FileBlocked {
		t.Fatalf("Scan() = %q, %v; want blocked with timeout error", state, err)
	}
}

func TestScanStreamTimeoutFailsClosed(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		versionConn, err := listener.Accept()
		if err != nil {
			return
		}
		if readCommand(versionConn, 9) {
			_, _ = fmt.Fprint(versionConn, "ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026", string([]byte{0}))
		}
		_ = versionConn.Close()
		scanConn, err := listener.Accept()
		if err != nil {
			return
		}
		defer scanConn.Close()
		if !readCommand(scanConn, 10) {
			return
		}
		for {
			var header [4]byte
			if _, err := io.ReadFull(scanConn, header[:]); err != nil {
				return
			}
			n := int(binary.BigEndian.Uint32(header[:]))
			if n == 0 {
				break
			}
			if !readCommand(scanConn, n) {
				return
			}
		}
		time.Sleep(100 * time.Millisecond) // Simulate a scanner that never completes.
	}()
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	a, err := New(Config{
		Address: listener.Addr().String(), Timeout: 25 * time.Millisecond,
		MaxBytes: 1024, MaxSignatureAge: 48 * time.Hour,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.Scan(context.Background(), []byte("synthetic"))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("synthetic ClamAV server did not finish its timeout exchange")
	}
	if err == nil || state != dv.FileBlocked {
		t.Fatalf("Scan() = %q, %v; want blocked with scan-timeout error", state, err)
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
		defer listener.Close()
		for request := 0; request < 2; request++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			if request == 0 {
				if !readCommand(conn, 9) {
					_ = conn.Close()
					return
				}
				_, _ = fmt.Fprint(conn, "ClamAV 1.5.4/28167/Wed Oct 28 06:24:17 2026", string([]byte{0}))
				_ = conn.Close()
				continue
			}
			if !readCommand(conn, 10) {
				_ = conn.Close()
				return
			}
			for {
				var header [4]byte
				if _, err := io.ReadFull(conn, header[:]); err != nil {
					_ = conn.Close()
					return
				}
				n := int(header[0])<<24 | int(header[1])<<16 | int(header[2])<<8 | int(header[3])
				if n == 0 {
					break
				}
				if !readCommand(conn, n) {
					_ = conn.Close()
					return
				}
			}
			_, _ = fmt.Fprint(conn, response, string([]byte{0}))
			_ = conn.Close()
		}
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
