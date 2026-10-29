package clamav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type Config struct {
	Address         string
	Timeout         time.Duration
	MaxBytes        int64
	MaxSignatureAge time.Duration
	Now             func() time.Time
}

type Adapter struct{ cfg Config }

func New(cfg Config) (*Adapter, error) {
	if strings.TrimSpace(cfg.Address) == "" || cfg.Timeout <= 0 || cfg.MaxBytes <= 0 || cfg.MaxSignatureAge <= 0 {
		return nil, errors.New("clamav: invalid configuration")
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Adapter{cfg: cfg}, nil
}

func (a *Adapter) Scan(ctx context.Context, content []byte) (dv.FileState, error) {
	return a.ScanReader(ctx, bytes.NewReader(content))
}

func (a *Adapter) ScanReader(ctx context.Context, content io.Reader) (dv.FileState, error) {
	fail := func(err error) (dv.FileState, error) { return dv.FileBlocked, err }
	if a == nil || a.cfg.Address == "" || content == nil {
		return fail(errors.New("clamav: unavailable configuration or content"))
	}
	version, err := a.signatureVersion(ctx)
	if err != nil {
		return fail(err)
	}
	if err := validateVersion(version, a.cfg.Now(), a.cfg.MaxSignatureAge); err != nil {
		return fail(err)
	}

	conn, stopCancel, err := a.dial(ctx)
	if err != nil {
		return fail(errors.New("clamav: scan connection failed"))
	}
	defer conn.Close()
	defer stopCancel()
	reader := bufio.NewReaderSize(conn, 4096)
	if _, err := io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return fail(fmt.Errorf("clamav: stream request failed: %w", err))
	}
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, readErr := content.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > a.cfg.MaxBytes {
				return fail(errors.New("clamav: content exceeds configured limit"))
			}
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(n))
			if _, err := conn.Write(size[:]); err != nil {
				return fail(fmt.Errorf("clamav: stream length write failed: %w", err))
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return fail(fmt.Errorf("clamav: stream content write failed: %w", err))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fail(errors.New("clamav: content read failed"))
		}
	}
	if total == 0 {
		return fail(errors.New("clamav: empty content is not scannable"))
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fail(fmt.Errorf("clamav: stream terminator failed: %w", err))
	}
	response, err := readClamdMessage(reader)
	if err != nil {
		return fail(errors.New("clamav: scan response unavailable"))
	}
	if strings.HasPrefix(response, "stream: ") && strings.HasSuffix(response, " OK") {
		return dv.FileClean, nil
	}
	if strings.HasPrefix(response, "stream: ") && strings.HasSuffix(response, " FOUND") {
		return dv.FileBlocked, nil
	}
	return fail(errors.New("clamav: malformed or error scan response"))
}

// ClamD handles one command per connection; version freshness is checked on a
// separate connection before the stream request is sent.
func (a *Adapter) signatureVersion(ctx context.Context) (string, error) {
	conn, stopCancel, err := a.dial(ctx)
	if err != nil {
		return "", errors.New("clamav: version connection failed")
	}
	defer conn.Close()
	defer stopCancel()
	if _, err := io.WriteString(conn, "zVERSION\x00"); err != nil {
		return "", errors.New("clamav: version request failed")
	}
	version, err := readClamdMessage(bufio.NewReaderSize(conn, 4096))
	if err != nil {
		return "", errors.New("clamav: version metadata unavailable")
	}
	return version, nil
}

func (a *Adapter) dial(ctx context.Context) (net.Conn, func() bool, error) {
	conn, err := (&net.Dialer{Timeout: a.cfg.Timeout}).DialContext(ctx, "tcp", a.cfg.Address)
	if err != nil {
		return nil, nil, err
	}
	deadline := time.Now().Add(a.cfg.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	return conn, stopCancel, nil
}

func readClamdMessage(reader *bufio.Reader) (string, error) {
	message, err := reader.ReadSlice(0)
	if err != nil {
		return "", err
	}
	if len(message) == 0 || message[len(message)-1] != 0 {
		return "", errors.New("clamav: unterminated response")
	}
	return strings.TrimSpace(strings.TrimSuffix(string(message), "\x00")), nil
}

func validateVersion(line string, now time.Time, maxAge time.Duration) error {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\x00"))
	if !strings.HasPrefix(line, "ClamAV ") {
		return errors.New("clamav: malformed version metadata")
	}
	parts := strings.Split(line, "/")
	if len(parts) < 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return errors.New("clamav: malformed version metadata")
	}
	updatedAt, err := time.Parse("Mon Jan 2 15:04:05 2006", strings.TrimSpace(parts[len(parts)-1]))
	if err != nil {
		return errors.New("clamav: malformed version date")
	}
	if maxAge <= 0 || updatedAt.After(now.Add(5*time.Minute)) || now.Sub(updatedAt) > maxAge {
		return errors.New("clamav: stale or invalid signature metadata")
	}
	return nil
}
