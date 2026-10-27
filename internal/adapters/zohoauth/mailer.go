// Package zohoauth sends authentication messages through an authenticated,
// TLS-protected SMTP submission endpoint. It never logs message content.
package zohoauth

import (
	"bytes"
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
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

//go:embed espacigo-logo.png
var espacigoLogoPNG []byte

type Config struct {
	Host, Port, Security, Username, PasswordFile, From, FromName, PublicOrigin string
}

type Mailer struct {
	config      Config
	password    string
	dialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	tlsConfig   *tls.Config
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

var zohoSubmissionHost = regexp.MustCompile(`^(smtp|smtppro)\.(zoho)\.(com|eu|in|com\.au|jp|com\.cn)$`)

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

func (m *Mailer) authFromName() string {
	if strings.TrimSpace(m.config.FromName) == "EspaciGo (pruebas)" {
		return "EspaciGo"
	}
	return m.config.FromName
}

func (m *Mailer) send(ctx context.Context, to, subject, body string) error {
	return m.sendRaw(ctx, m.authFromName(), to, subject, "text/plain; charset=utf-8", []byte(body))
}

func (m *Mailer) sendMultipart(ctx context.Context, to, subject, textBody, htmlBody string) error {
	var relatedBuf bytes.Buffer
	relatedWriter := multipart.NewWriter(&relatedBuf)
	logoCID := "espacigo-logo-" + relatedWriter.Boundary() + "@inline"
	htmlBody = strings.Replace(htmlBody, "cid:espacigo-logo@inline", "cid:"+logoCID, 1)

	var altBuf bytes.Buffer
	altWriter := multipart.NewWriter(&altBuf)

	textHeader := "Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n"
	textPart, err := altWriter.CreatePart(mailHeaderFromRaw(textHeader))
	if err != nil {
		return identity.ErrDelivery
	}
	qpText := quotedprintable.NewWriter(textPart)
	if _, err := qpText.Write([]byte(textBody)); err != nil {
		return identity.ErrDelivery
	}
	if err := qpText.Close(); err != nil {
		return identity.ErrDelivery
	}

	htmlHeader := "Content-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n"
	htmlPart, err := altWriter.CreatePart(mailHeaderFromRaw(htmlHeader))
	if err != nil {
		return identity.ErrDelivery
	}
	qpHtml := quotedprintable.NewWriter(htmlPart)
	if _, err := qpHtml.Write([]byte(htmlBody)); err != nil {
		return identity.ErrDelivery
	}
	if err := qpHtml.Close(); err != nil {
		return identity.ErrDelivery
	}
	if err := altWriter.Close(); err != nil {
		return identity.ErrDelivery
	}

	altPartHeader := fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altWriter.Boundary())
	altRelPart, err := relatedWriter.CreatePart(mailHeaderFromRaw(altPartHeader))
	if err != nil {
		return identity.ErrDelivery
	}
	if _, err := altRelPart.Write(altBuf.Bytes()); err != nil {
		return identity.ErrDelivery
	}

	imgHeader := fmt.Sprintf("Content-Type: image/png; name=\"espacigo-logo.png\"\r\nContent-Transfer-Encoding: base64\r\nContent-ID: <%s>\r\nContent-Disposition: inline; filename=\"espacigo-logo.png\"\r\n\r\n", logoCID)
	imgPart, err := relatedWriter.CreatePart(mailHeaderFromRaw(imgHeader))
	if err != nil {
		return identity.ErrDelivery
	}
	if err := writeBase64Lines(imgPart, espacigoLogoPNG); err != nil {
		return identity.ErrDelivery
	}

	if err := relatedWriter.Close(); err != nil {
		return identity.ErrDelivery
	}

	contentType := fmt.Sprintf("multipart/related; boundary=\"%s\"; type=\"multipart/alternative\"", relatedWriter.Boundary())
	return m.sendRaw(ctx, m.authFromName(), to, subject, contentType, relatedBuf.Bytes())
}

func writeBase64Lines(w io.Writer, data []byte) error {
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		if _, err := io.WriteString(w, enc[:76]+"\r\n"); err != nil {
			return err
		}
		enc = enc[76:]
	}
	if len(enc) > 0 {
		if _, err := io.WriteString(w, enc+"\r\n"); err != nil {
			return err
		}
	}
	return nil
}

func mailHeaderFromRaw(raw string) map[string][]string {
	lines := strings.Split(strings.TrimRight(raw, "\r\n"), "\r\n")
	hdr := make(map[string][]string)
	for _, l := range lines {
		parts := strings.SplitN(l, ": ", 2)
		if len(parts) == 2 {
			hdr[parts[0]] = []string{parts[1]}
		}
	}
	return hdr
}

func (m *Mailer) sendRaw(ctx context.Context, fromName, to, subject, contentType string, content []byte) error {
	if m == nil || identity.ValidateEmail(to) != nil || strings.ContainsAny(subject, "\r\n") {
		return identity.ErrDelivery
	}
	address := net.JoinHostPort(m.config.Host, m.config.Port)
	tlsConfig := m.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if m.dialContext != nil {
		conn, err = m.dialContext(ctx, "tcp", address)
	} else if m.config.Security == "tls" {
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
	from := (&mail.Address{Name: fromName, Address: m.config.From}).String()
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
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	_, err = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n", from, toHeader, encodedSubject, contentType)
	if err != nil {
		_ = writer.Close()
		return identity.ErrDelivery
	}
	if _, err = writer.Write(content); err != nil {
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
	link := m.actionURL(d.TokenID, d.Token)
	expiryUTC := d.ExpiresAt.UTC().Format("2006-01-02 15:04:05 UTC")
	subject := "Verifica tu correo y activa tu cuenta en EspaciGo"

	textBody := fmt.Sprintf(
		"EspaciGo\r\n\r\nVerifica tu correo y activa tu cuenta en EspaciGo\r\n\r\nUsa el siguiente enlace para verificar tu correo y activar tu cuenta:\r\n%s\r\n\r\nEste enlace es de un solo uso y vence el %s.\r\n\r\nSi no solicitaste esta acción, puedes ignorar este mensaje. No compartas este enlace.\r\n",
		link, expiryUTC,
	)

	htmlBody := renderAuthEmailHTML(authEmailData{
		Title:        "Verifica tu correo y activa tu cuenta en EspaciGo",
		Lead:         "Gracias por registrarte en EspaciGo. Confirma tu dirección de correo para activar tu cuenta y comenzar a gestionar o reservar espacios.",
		ButtonText:   "Verificar correo y activar cuenta",
		ButtonURL:    link,
		ExpiryUTC:    expiryUTC,
		HelpNotice:   "Si el botón no funciona, copia y pega el siguiente enlace en tu navegador:",
		IgnoreNotice: "Si no solicitaste esta acción, puedes ignorar este mensaje. No compartas este enlace.",
	})

	return m.sendMultipart(ctx, d.Email, subject, textBody, htmlBody)
}

func (m *Mailer) SendRecovery(ctx context.Context, d identity.RecoveryDelivery) error {
	link := m.config.PublicOrigin + "/recuperar-acceso#" + url.Values{"token_id": {d.TokenID}, "token": {string(d.Token)}}.Encode()
	expiryUTC := d.ExpiresAt.UTC().Format("2006-01-02 15:04:05 UTC")
	subject := "Restablece tu contraseña de EspaciGo"

	textBody := fmt.Sprintf(
		"EspaciGo\r\n\r\nRestablece tu contraseña de EspaciGo\r\n\r\nRecibimos una solicitud para restablecer la contraseña de tu cuenta. Accede al siguiente enlace:\r\n%s\r\n\r\nEste enlace es de un solo uso y vence el %s.\r\n\r\nSi no solicitaste esta acción, puedes ignorar este mensaje. No compartas este enlace.\r\n",
		link, expiryUTC,
	)

	htmlBody := renderAuthEmailHTML(authEmailData{
		Title:        "Restablece tu contraseña de EspaciGo",
		Lead:         "Hemos recibido una solicitud para cambiar tu contraseña en EspaciGo. Haz clic en el botón inferior para definir una nueva clave de acceso.",
		ButtonText:   "Restablecer contraseña",
		ButtonURL:    link,
		ExpiryUTC:    expiryUTC,
		HelpNotice:   "Si el botón no funciona, copia y pega el siguiente enlace en tu navegador:",
		IgnoreNotice: "Si no solicitaste esta acción, puedes ignorar este mensaje. No compartas este enlace.",
	})

	return m.sendMultipart(ctx, d.Email, subject, textBody, htmlBody)
}

func (m *Mailer) SendPasswordChanged(ctx context.Context, email string) error {
	return m.send(ctx, email, "Actualización de clave de EspaciGo", "La clave de tu cuenta fue actualizada. Si no reconoces este cambio, contacta al equipo de soporte.")
}

func (m *Mailer) SendLoginAlert(ctx context.Context, email string, until time.Time) error {
	return m.send(ctx, email, "Aviso de acceso a EspaciGo", "El acceso a tu cuenta está temporalmente bloqueado hasta "+until.UTC().Format(time.RFC3339)+".")
}

type authEmailData struct {
	Title        string
	Lead         string
	ButtonText   string
	ButtonURL    string
	ExpiryUTC    string
	HelpNotice   string
	IgnoreNotice string
}

func renderAuthEmailHTML(data authEmailData) string {
	cleanURL := html.EscapeString(data.ButtonURL)
	cleanTitle := html.EscapeString(data.Title)
	cleanLead := html.EscapeString(data.Lead)
	cleanBtnText := html.EscapeString(data.ButtonText)
	cleanExpiry := html.EscapeString(data.ExpiryUTC)
	cleanHelp := html.EscapeString(data.HelpNotice)
	cleanIgnore := html.EscapeString(data.IgnoreNotice)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="color-scheme" content="light">
  <title>%s</title>
</head>
<body style="margin: 0; padding: 0; background-color: #FBF7F2; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #332720; -webkit-font-smoothing: antialiased;">
  <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="background-color: #FBF7F2; padding: 32px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" border="0" cellspacing="0" cellpadding="0" style="max-width: 580px; background-color: #FFFFFF; border-radius: 12px; border: 1px solid #DED5CC; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 20px 32px; border-bottom: 1px solid #DED5CC; text-align: left;">
              <table role="presentation" border="0" cellspacing="0" cellpadding="0" style="border-collapse: collapse;">
                <tr>
                  <td style="vertical-align: middle; padding-right: 12px; line-height: 0;">
                    <img src="cid:espacigo-logo@inline" alt="EspaciGo" width="40" height="40" style="display: block; width: 40px; height: 40px; border: 0;" />
                  </td>
                  <td style="vertical-align: middle;">
                    <span style="font-size: 24px; font-weight: 800; letter-spacing: -0.5px; color: #332720; text-decoration: none; white-space: nowrap;">
                      Espaci<span style="color: #A44731;">Go</span>
                    </span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Main Content -->
          <tr>
            <td style="padding: 32px; text-align: left;">
              <h1 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 700; color: #332720; line-height: 1.35;">
                %s
              </h1>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #66574E;">
                %s
              </p>
              <!-- CTA Button -->
              <div style="margin: 28px 0; text-align: left;">
                <a href="%s" style="display: inline-block; background-color: #A44731; color: #FFFFFF; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 28px; border-radius: 8px; box-shadow: 0 2px 4px rgba(164, 71, 49, 0.2);" target="_blank">
                  %s
                </a>
              </div>
              <!-- Expiry Callout -->
              <div style="background-color: #F5E3DB; border-left: 4px solid #A44731; padding: 12px 16px; border-radius: 4px; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 13px; color: #332720; line-height: 1.5;">
                  <strong>Vencimiento:</strong> Este enlace es válido de un solo uso y expira el <strong>%s</strong>.
                </p>
              </div>
              <!-- Fallback Link -->
              <p style="margin: 0 0 8px 0; font-size: 13px; color: #66574E; line-height: 1.5;">
                %s
              </p>
              <p style="margin: 0 0 24px 0; font-size: 12px; word-break: break-all; color: #A44731; line-height: 1.4;">
                <a href="%s" style="color: #A44731; text-decoration: underline;">%s</a>
              </p>
              <hr style="border: 0; border-top: 1px solid #DED5CC; margin: 24px 0;">
              <!-- Security Note -->
              <p style="margin: 0; font-size: 12px; color: #66574E; line-height: 1.5;">
                %s
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 20px 32px; background-color: #FBF7F2; border-top: 1px solid #DED5CC; text-align: left;">
              <p style="margin: 0; font-size: 12px; color: #66574E; line-height: 1.5;">
                &copy; EspaciGo. Plataforma integral para la gestión y reserva de espacios.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, cleanTitle, cleanTitle, cleanLead, cleanURL, cleanBtnText, cleanExpiry, cleanHelp, cleanURL, cleanURL, cleanIgnore)
}

func (m *Mailer) SendLocalBookingNotice(ctx context.Context, email, subject, body string) error {
	return m.sendRaw(ctx, m.config.FromName, email, subject+" (EspaciGo pruebas)", "text/plain; charset=utf-8", []byte(body))
}
