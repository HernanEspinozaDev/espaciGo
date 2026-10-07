// Package devauth contains explicit single-process development adapters.
// Their quota is volatile and their SMTP destination must be a local capture inbox.
package devauth

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type IPLimiter struct {
	mu    sync.Mutex
	calls map[string][]time.Time
	limit int
}

func NewIPLimiter(limit int) *IPLimiter {
	return &IPLimiter{calls: make(map[string][]time.Time), limit: limit}
}
func (l *IPLimiter) AllowVerification(ctx context.Context, ip string, at time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// Prune inactive keys too, preventing unbounded storage for this local adapter.
	for key, times := range l.calls {
		var recent []time.Time
		for _, old := range times {
			if old.After(at.Add(-time.Hour)) {
				recent = append(recent, old)
			}
		}
		if len(recent) == 0 {
			delete(l.calls, key)
		} else {
			l.calls[key] = recent
		}
	}
	if l.limit < 1 || len(l.calls[ip]) >= l.limit {
		return false, nil
	}
	l.calls[ip] = append(l.calls[ip], at)
	return true, nil
}

type Mailer struct{ Address string }

func (m Mailer) send(ctx context.Context, email, subject, body string) error {
	// Validate the recipient before constructing its display header; no SMTP/body logs.
	if identity.ValidateEmail(email) != nil {
		return identity.ErrDelivery
	}
	recipient := (&mail.Address{Address: strings.TrimSpace(email)}).String()
	// Use a deadline and no debug/SMTP logging.
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", m.Address)
	if err != nil {
		return identity.ErrDelivery
	}
	defer conn.Close()
	deadline := time.Now().Add(5 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = conn.SetDeadline(deadline)
	host, _, err := net.SplitHostPort(m.Address)
	if err != nil {
		return identity.ErrDelivery
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return identity.ErrDelivery
	}
	defer client.Close()
	if err = client.Mail("dev@espacigo.invalid"); err != nil {
		return identity.ErrDelivery
	}
	if err = client.Rcpt(strings.TrimSpace(email)); err != nil {
		return identity.ErrDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return identity.ErrDelivery
	}
	_, err = fmt.Fprintf(writer, "From: EspaciGo local <dev@espacigo.invalid>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", recipient, subject, body)
	if err != nil {
		return identity.ErrDelivery
	}
	if writer.Close() != nil {
		return identity.ErrDelivery
	}
	if client.Quit() != nil {
		return identity.ErrDelivery
	}
	return nil
}
func (m Mailer) SendVerification(ctx context.Context, d identity.VerificationDelivery) error {
	return m.send(ctx, d.Email, "EspaciGo - verifica tu correo (desarrollo)", "Correo sintetico de desarrollo; no es envio productivo.\r\nToken ID: "+d.TokenID+"\r\nToken: "+string(d.Token)+"\r\nExpira: "+d.ExpiresAt.UTC().Format(time.RFC3339)+"\r\nCopia ambos valores en el formulario de verificacion del mock local.")
}
func (m Mailer) SendRecovery(ctx context.Context, d identity.RecoveryDelivery) error {
	return m.send(ctx, d.Email, "EspaciGo - recupera tu clave (desarrollo)", "Correo sintetico de desarrollo; no es envio productivo.\r\nToken ID: "+d.TokenID+"\r\nToken: "+string(d.Token)+"\r\nExpira: "+d.ExpiresAt.UTC().Format(time.RFC3339)+"\r\nCopia ambos valores en el formulario de recuperacion del mock local.")
}
func (m Mailer) SendPasswordChanged(ctx context.Context, email string) error {
	return m.send(ctx, email, "EspaciGo - clave actualizada (desarrollo)", "La clave de tu cuenta EspaciGo local fue actualizada. Todas las sesiones anteriores fueron revocadas. Si no hiciste este cambio, contacta al equipo de desarrollo.")
}
func (m Mailer) SendLoginAlert(ctx context.Context, email string, until time.Time) error {
	return m.send(ctx, email, "EspaciGo - bloqueo de login (desarrollo)", "Login bloqueado hasta "+until.UTC().Format(time.RFC3339)+" tras "+strconv.Itoa(5)+" intentos incorrectos.")
}

func (m Mailer) SendLocalBookingNotice(ctx context.Context, email, subject, body string) error {
	return m.send(ctx, email, subject+" (ensayo local)", body)
}
