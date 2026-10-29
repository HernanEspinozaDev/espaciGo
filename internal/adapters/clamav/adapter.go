package clamav

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
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
	return a.ScanReader(ctx, strings.NewReader(string(content)))
}
func (a *Adapter) ScanReader(ctx context.Context, content io.Reader) (dv.FileState, error) {
	fail := func(err error) (dv.FileState, error) { return dv.FileBlocked, err }
	if a == nil || a.cfg.Address == "" {
		return fail(errors.New("clamav: unavailable configuration"))
	}
	conn, err := (&net.Dialer{Timeout: a.cfg.Timeout}).DialContext(ctx, "tcp", a.cfg.Address)
	if err != nil {
		return fail(errors.New("clamav: connection failed"))
	}
	defer conn.Close()
	deadline := time.Now().Add(a.cfg.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	if _, err = io.WriteString(conn, "zVERSION\x00"); err != nil {
		return fail(errors.New("clamav: version request failed"))
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fail(errors.New("clamav: version metadata unavailable"))
	}
	if err = validateVersion(strings.TrimSpace(line), a.cfg.Now(), a.cfg.MaxSignatureAge); err != nil {
		return fail(err)
	}
	if _, err = io.WriteString(conn, "zINSTREAM\x00"); err != nil {
		return fail(errors.New("clamav: stream request failed"))
	}
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, e := content.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > a.cfg.MaxBytes {
				return fail(errors.New("clamav: content exceeds configured limit"))
			}
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(n))
			if _, err = conn.Write(size[:]); err != nil {
				return fail(errors.New("clamav: stream failed"))
			}
			if _, err = conn.Write(buf[:n]); err != nil {
				return fail(errors.New("clamav: stream failed"))
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return fail(errors.New("clamav: content read failed"))
		}
	}
	if _, err = conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fail(errors.New("clamav: stream terminator failed"))
	}
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fail(errors.New("clamav: scan response unavailable"))
	}
	response = strings.TrimSpace(response)
	if strings.HasSuffix(response, "stream: OK") {
		return dv.FileClean, nil
	}
	if strings.Contains(response, "stream: ") && strings.HasSuffix(response, " FOUND") {
		return dv.FileBlocked, nil
	}
	return fail(fmt.Errorf("clamav: malformed scan response"))
}

func validateVersion(line string, now time.Time, maxAge time.Duration) error {
	const marker = " ("
	i := strings.LastIndex(line, marker)
	if i <= 0 || !strings.HasSuffix(line, ")") {
		return errors.New("clamav: malformed version metadata")
	}
	date := strings.TrimSuffix(line[i+len(marker):], ")")
	parsed, err := time.Parse("02 Jan 2006", date)
	if err != nil {
		return errors.New("clamav: malformed version date")
	}
	if strings.TrimSpace(line[:i]) == "" || parsed.After(now.Add(24*time.Hour)) || now.Sub(parsed) > maxAge {
		return errors.New("clamav: stale or invalid signature metadata")
	}
	if _, err := strconv.Atoi(parsed.Format("2006")); err != nil {
		return errors.New("clamav: malformed version metadata")
	}
	return nil
}
