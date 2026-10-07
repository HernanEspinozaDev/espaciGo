package devauth

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalQuotaIsAtomicAndExpires(t *testing.T) {
	limiter := NewIPLimiter(3)
	now := time.Now()
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := limiter.AllowVerification(context.Background(), "192.0.2.1", now)
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 3 {
		t.Fatal("local quota lost concurrent updates")
	}
	if ok, _ := limiter.AllowVerification(context.Background(), "192.0.2.1", now.Add(time.Hour)); !ok {
		t.Fatal("local quota did not expire")
	}
	if ok, _ := limiter.AllowVerification(context.Background(), "192.0.2.2", now); !ok {
		t.Fatal("different IP shared quota")
	}
}

func TestPasswordChangedMailUsesGenericLocalNotice(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan string, 1)
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		write := func(value string) error {
			if _, err := fmt.Fprint(writer, value); err != nil {
				return err
			}
			return writer.Flush()
		}
		if err := write("220 local smtp\r\n"); err != nil {
			serverErr <- err
			return
		}
		var body strings.Builder
		inBody := false
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				serverErr <- readErr
				return
			}
			if inBody {
				if line == ".\r\n" {
					inBody = false
					received <- body.String()
					if err := write("250 queued\r\n"); err != nil {
						serverErr <- err
						return
					}
					continue
				}
				body.WriteString(line)
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				if err := write("250-localhost\r\n250 OK\r\n"); err != nil {
					serverErr <- err
					return
				}
			case strings.HasPrefix(line, "MAIL FROM:") || strings.HasPrefix(line, "RCPT TO:"):
				if err := write("250 OK\r\n"); err != nil {
					serverErr <- err
					return
				}
			case strings.TrimSpace(line) == "DATA":
				inBody = true
				if err := write("354 end data\r\n"); err != nil {
					serverErr <- err
					return
				}
			case strings.TrimSpace(line) == "QUIT":
				_ = write("221 bye\r\n")
				serverErr <- nil
				return
			default:
				serverErr <- fmt.Errorf("unexpected SMTP command")
				return
			}
		}
	}()
	secretValues := []string{"Synthetic#123", "bcrypt-hash-must-not-appear", "recovery-token-must-not-appear"}
	if err := (Mailer{Address: listener.Addr().String()}).SendPasswordChanged(context.Background(), "person@example.invalid"); err != nil {
		t.Fatal(err)
	}
	message := <-received
	if !strings.Contains(message, "Subject: EspaciGo - clave actualizada (desarrollo)") || !strings.Contains(message, "Todas las sesiones anteriores fueron revocadas") {
		t.Fatalf("generic password-change notice incomplete: %q", message)
	}
	for _, secret := range secretValues {
		if strings.Contains(message, secret) {
			t.Fatalf("notice contains secret material %q", secret)
		}
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}
