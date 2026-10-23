// Package zohoauth sends authentication messages through an authenticated,
// TLS-protected SMTP submission endpoint. It never logs message content.
package zohoauth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type Config struct {
	Host, Port, Security, Username, PasswordFile, From, FromName, PublicOrigin string
}

type Mailer struct {
	config   Config
	password string
}

// UnavailableMailer keeps the Cloud test API available while preventing
// fallback to the local Mailpit sink when SMTP has not yet been configured.
type UnavailableMailer struct{}

func (UnavailableMailer) SendVerification(context.Context, identity.VerificationDelivery) error {
	return identity.ErrDelivery
}
func (UnavailableMailer) SendRecovery(context.Context, identity.RecoveryDelivery) error {
	return identity.ErrDelivery
}
func (UnavailableMailer) SendPasswordChanged(context.Context, string) error {
	return identity.ErrDelivery
}
func (UnavailableMailer) SendLoginAlert(context.Context, string, time.Time) error {
	return identity.ErrDelivery
}
func (UnavailableMailer) SendLocalBookingNotice(context.Context, string, string, string) error {
	return identity.ErrDelivery
}

var zohoSubmissionHost = regexp.MustCompile(`^(smtp|smtppro)\.zoho\.(com|eu|in|com\.au|jp|com\.cn)$`)

func New(config Config) (*Mailer, error) {
	if !zohoSubmissionHost.MatchString(config.Host) || config.Username != "contacto@hotusoft.com" || config.From != config.Username || (config.Port != "587" && config.Port != "465") ||
		(config.Security != "starttls" && config.Security != "tls") ||
		(config.Port == "587" && config.Security != "starttls") || (config.Port == "465" && config.Security != "tls") ||
		identity.ValidateEmail(config.Username) != nil || identity.ValidateEmail(config.From) != nil ||
		config.PasswordFile == "" || !validOrigin(config.PublicOrigin) {
		return nil, errors.New("zoho SMTP configuration is incomplete or invalid")
	}
	secret, err := os.ReadFile(config.PasswordFile)
	if err != nil {
		return nil, errors.New("zoho SMTP credential file unavailable")
	}
	password := strings.TrimSpace(string(secret))
	if password == "" {
		return nil, errors.New("zoho SMTP credential file is empty")
	}
	return &Mailer{config: config, password: password}, nil
}

func validOrigin(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}

func (m *Mailer) send(ctx context.Context, to, subject, body string) error {
	if m == nil || identity.ValidateEmail(to) != nil {
		return identity.ErrDelivery
	}
	address := net.JoinHostPort(m.config.Host, m.config.Port)
	tlsConfig := &tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if m.config.Security == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return identity.ErrDelivery
	}
	defer conn.Close()
	deadline := time.Now().Add(15 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	_ = conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, m.config.Host)
	if err != nil {
		return identity.ErrDelivery
	}
	defer client.Close()
	if m.config.Security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok || client.StartTLS(tlsConfig) != nil {
			return identity.ErrDelivery
		}
	}
	if err = client.Auth(smtp.PlainAuth("", m.config.Username, m.password, m.config.Host)); err != nil {
		return identity.ErrDelivery
	}
	from := (&mail.Address{Name: m.config.FromName, Address: m.config.From}).String()
	toHeader := (&mail.Address{Address: strings.TrimSpace(to)}).String()
	if err = client.Mail(m.config.From); err != nil {
		return identity.ErrDelivery
	}
	if err = client.Rcpt(strings.TrimSpace(to)); err != nil {
		return identity.ErrDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return identity.ErrDelivery
	}
	_, err = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", from, toHeader, subject, body)
	if err != nil {
		_ = writer.Close()
		return identity.ErrDelivery
	}
	if writer.Close() != nil || client.Quit() != nil {
		return identity.ErrDelivery
	}
	return nil
}

func (m *Mailer) actionURL(id string, token identity.Secret) string {
	fragment := url.Values{"token_id": {id}, "token": {string(token)}}.Encode()
	return m.config.PublicOrigin + "/verificar-correo#" + fragment
}
func (m *Mailer) SendVerification(ctx context.Context, d identity.VerificationDelivery) error {
	return m.send(ctx, d.Email, "Verifica tu correo de EspaciGo (pruebas)", "Verifica tu cuenta con este enlace seguro de un solo uso, válido hasta "+d.ExpiresAt.UTC().Format(time.RFC3339)+":\r\n"+m.actionURL(d.TokenID, d.Token))
}
func (m *Mailer) SendRecovery(ctx context.Context, d identity.RecoveryDelivery) error {
	return m.send(ctx, d.Email, "Recupera tu acceso a EspaciGo (pruebas)", "Usa este enlace seguro de un solo uso antes de "+d.ExpiresAt.UTC().Format(time.RFC3339)+":\r\n"+m.config.PublicOrigin+"/recuperar-acceso#"+url.Values{"token_id": {d.TokenID}, "token": {string(d.Token)}}.Encode())
}
func (m *Mailer) SendPasswordChanged(ctx context.Context, email string) error {
	return m.send(ctx, email, "Actualización de clave de EspaciGo (pruebas)", "La clave de tu cuenta fue actualizada. Si no reconoces este cambio, contacta al equipo de soporte.")
}
func (m *Mailer) SendLoginAlert(ctx context.Context, email string, until time.Time) error {
	return m.send(ctx, email, "Aviso de acceso a EspaciGo (pruebas)", "El acceso a tu cuenta está temporalmente bloqueado hasta "+until.UTC().Format(time.RFC3339)+".")
}

func (m *Mailer) SendLocalBookingNotice(ctx context.Context, email, subject, body string) error {
	return m.send(ctx, email, subject+" (EspaciGo pruebas)", body)
}
