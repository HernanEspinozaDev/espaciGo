package zohoauth

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestNewRequiresExplicitTLSAndReadsMountedCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("application-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Host: "smtp.zoho.com", Port: "587", Security: "starttls", Username: "contacto@hotusoft.com", PasswordFile: path, From: "contacto@hotusoft.com", FromName: "EspaciGo (pruebas)", PublicOrigin: "https://espacigo.testingpage.store"}
	mailer, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if mailer.password != "application-secret" {
		t.Fatal("mounted SMTP credential was not read")
	}
	if url := mailer.actionURL("verification-id", identity.Secret("one-time-token")); url != "https://espacigo.testingpage.store/verificar-correo#token=one-time-token&token_id=verification-id" {
		t.Fatalf("verification link=%q", url)
	}
}

func TestNewRejectsInsecureOrMismatchedSMTPConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("application-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	base := Config{Host: "smtp.zoho.com", Port: "587", Security: "starttls", Username: "contacto@hotusoft.com", PasswordFile: path, From: "contacto@hotusoft.com", FromName: "EspaciGo (pruebas)", PublicOrigin: "https://espacigo.testingpage.store"}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Security = "none" },
		func(c *Config) { c.Port = "25" },
		func(c *Config) { c.Host = "mailpit.internal" },
		func(c *Config) { c.PublicOrigin = "https://espacigo.testingpage.store/path" },
	} {
		cfg := base
		mutate(&cfg)
		if _, err := New(cfg); err == nil {
			t.Fatalf("accepted invalid SMTP config: %+v", cfg)
		}
	}
}

type fakeSMTPServer struct {
	listener  net.Listener
	mu        sync.Mutex
	messages  [][]byte
	clientTLS *tls.Config
}

func startFakeSMTPServer(t *testing.T, expectedUser, expectedPass string) *fakeSMTPServer {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "smtp.zoho.com"},
		DNSNames:     []string{"smtp.zoho.com"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{derBytes},
			PrivateKey:  priv,
		}},
	}
	rootPool := x509.NewCertPool()
	rootPool.AddCert(cert)
	clientTLS := &tls.Config{
		ServerName: "smtp.zoho.com",
		RootCAs:    rootPool,
		MinVersion: tls.VersionTLS12,
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake smtp: %v", err)
	}

	srv := &fakeSMTPServer{
		listener:  lis,
		clientTLS: clientTLS,
	}

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go handleFakeSMTPConn(conn, serverTLS, expectedUser, expectedPass, srv)
		}
	}()

	t.Cleanup(func() {
		_ = lis.Close()
	})
	return srv
}

func handleFakeSMTPConn(conn net.Conn, serverTLS *tls.Config, expUser, expPass string, srv *fakeSMTPServer) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	_, _ = fmt.Fprintf(conn, "220 127.0.0.1 EspaciGo SMTP Mock Ready\r\n")
	tlsActive := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO") {
			if !tlsActive {
				_, _ = fmt.Fprintf(conn, "250-127.0.0.1\r\n250-STARTTLS\r\n250-AUTH PLAIN\r\n250 8BITMIME\r\n")
			} else {
				_, _ = fmt.Fprintf(conn, "250-127.0.0.1\r\n250-AUTH PLAIN\r\n250 8BITMIME\r\n")
			}
		} else if upper == "STARTTLS" {
			_, _ = fmt.Fprintf(conn, "220 2.0.0 Ready to start TLS\r\n")
			tlsConn := tls.Server(conn, serverTLS)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			tlsActive = true
		} else if strings.HasPrefix(upper, "AUTH PLAIN") {
			parts := strings.Split(trimmed, " ")
			var payload string
			if len(parts) > 2 {
				payload = parts[2]
			} else {
				_, _ = fmt.Fprintf(conn, "334 \r\n")
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				payload = strings.TrimRight(line, "\r\n")
			}
			rawAuth, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				_, _ = fmt.Fprintf(conn, "501 5.5.2 Cannot decode auth\r\n")
				continue
			}
			authTokens := bytes.Split(rawAuth, []byte{0})
			if len(authTokens) != 3 || string(authTokens[1]) != expUser || string(authTokens[2]) != expPass {
				_, _ = fmt.Fprintf(conn, "535 5.7.8 Authentication credentials invalid\r\n")
				continue
			}
			_, _ = fmt.Fprintf(conn, "235 2.7.0 Authentication successful\r\n")
		} else if strings.HasPrefix(upper, "MAIL FROM:") {
			_, _ = fmt.Fprintf(conn, "250 2.1.0 Sender OK\r\n")
		} else if strings.HasPrefix(upper, "RCPT TO:") {
			_, _ = fmt.Fprintf(conn, "250 2.1.5 Recipient OK\r\n")
		} else if upper == "DATA" {
			_, _ = fmt.Fprintf(conn, "354 End data with <CR><LF>.<CR><LF>\r\n")
			var msgBuf bytes.Buffer
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" || dataLine == ".\n" {
					break
				}
				msgBuf.WriteString(dataLine)
			}
			srv.mu.Lock()
			srv.messages = append(srv.messages, msgBuf.Bytes())
			srv.mu.Unlock()
			_, _ = fmt.Fprintf(conn, "250 2.0.0 Ok: queued\r\n")
		} else if upper == "QUIT" {
			_, _ = fmt.Fprintf(conn, "221 2.0.0 Bye\r\n")
			return
		} else {
			_, _ = fmt.Fprintf(conn, "500 5.5.1 Command unrecognized\r\n")
		}
	}
}

func (s *fakeSMTPServer) LastMessage() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return nil
	}
	return s.messages[len(s.messages)-1]
}

type parsedEmail struct {
	rawSubject      string
	subject         string
	from            string
	to              string
	textPart        string
	htmlPart        string
	hasLogo         bool
	logoRawCID      string
	logoCID         string
	logoContentType string
	logoFilename    string
	logoDisposition string
	logoBytes       []byte
	relatedType     string
	rootMediaType   string
}

func parseMultipartEmail(t *testing.T, raw []byte) parsedEmail {
	t.Helper()
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse mail message: %v", err)
	}

	rawSubject := msg.Header.Get("Subject")
	dec := new(mime.WordDecoder)
	subject, err := dec.DecodeHeader(rawSubject)
	if err != nil {
		subject = rawSubject
	}

	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	if mediaType != "multipart/related" {
		t.Fatalf("expected multipart/related, got %q", mediaType)
	}
	relatedType := params["type"]

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var textPart, htmlPart string
	var hasLogo bool
	var logoRawCID, logoCID, logoContentType, logoFilename, logoDisposition string
	var logoBytes []byte
	var rootMediaType string
	partIndex := 0

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading related next: %v", err)
		}
		pType, pParams, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse part content-type: %v", err)
		}
		if partIndex == 0 {
			rootMediaType = pType
		}
		partIndex++

		if pType == "multipart/alternative" {
			subReader := multipart.NewReader(part, pParams["boundary"])
			for {
				subPart, err := subReader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("reading alternative next: %v", err)
				}
				subType, _, _ := mime.ParseMediaType(subPart.Header.Get("Content-Type"))
				cte := strings.ToLower(subPart.Header.Get("Content-Transfer-Encoding"))
				var bodyBytes []byte
				if cte == "quoted-printable" {
					bodyBytes, err = io.ReadAll(quotedprintable.NewReader(subPart))
				} else {
					bodyBytes, err = io.ReadAll(subPart)
				}
				if err != nil {
					t.Fatalf("read alternative part body: %v", err)
				}
				if subType == "text/plain" {
					textPart = string(bodyBytes)
				} else if subType == "text/html" {
					htmlPart = string(bodyBytes)
				}
			}
		} else if strings.HasPrefix(pType, "image/") {
			hasLogo = true
			logoContentType = pType
			logoRawCID = part.Header.Get("Content-ID")
			logoCID = strings.Trim(logoRawCID, "<>")
			disp, dispParams, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
			logoDisposition = disp
			logoFilename = dispParams["filename"]
			cte := strings.ToLower(part.Header.Get("Content-Transfer-Encoding"))
			if cte == "base64" {
				logoBytes, err = io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
			} else {
				logoBytes, err = io.ReadAll(part)
			}
			if err != nil {
				t.Fatalf("read image body: %v", err)
			}
		}
	}

	return parsedEmail{
		rawSubject:      rawSubject,
		subject:         subject,
		from:            msg.Header.Get("From"),
		to:              msg.Header.Get("To"),
		textPart:        textPart,
		htmlPart:        htmlPart,
		hasLogo:         hasLogo,
		logoRawCID:      logoRawCID,
		logoCID:         logoCID,
		logoContentType: logoContentType,
		logoFilename:    logoFilename,
		logoDisposition: logoDisposition,
		logoBytes:       logoBytes,
		relatedType:     relatedType,
		rootMediaType:   rootMediaType,
	}
}

func TestSendVerificationBrandedEmail(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")

	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo (pruebas)", // baseline fixture with legacy mark
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}

	expiry := time.Date(2026, 10, 27, 18, 30, 0, 0, time.UTC)
	delivery := identity.VerificationDelivery{
		AccountID: "acc-42",
		TokenID:   "verif-tok-99",
		Email:     "arrendatario@example.com",
		Token:     identity.Secret("sec-token-xyz"),
		ExpiresAt: expiry,
	}

	err := mailer.SendVerification(context.Background(), delivery)
	if err != nil {
		t.Fatalf("SendVerification returned unexpected error: %v", err)
	}

	raw := srv.LastMessage()
	if raw == nil {
		t.Fatal("fake SMTP server received no message")
	}

	email := parseMultipartEmail(t, raw)

	// Assert From address header parses cleanly without 'pruebas' in Name
	fromAddr, err := mail.ParseAddress(email.from)
	if err != nil {
		t.Fatalf("parse From header error: %v", err)
	}
	if fromAddr.Name != "EspaciGo" {
		t.Errorf("expected From name 'EspaciGo', got %q", fromAddr.Name)
	}
	if fromAddr.Address != "contacto@hotusoft.com" {
		t.Errorf("expected From address 'contacto@hotusoft.com', got %q", fromAddr.Address)
	}

	// Assert exact subject decode (ASCII)
	wantSubject := "Verifica tu correo y activa tu cuenta en EspaciGo"
	if email.rawSubject != mime.QEncoding.Encode("utf-8", wantSubject) {
		t.Errorf("subject was not Q-encoded: got %q, want %q", email.rawSubject, mime.QEncoding.Encode("utf-8", wantSubject))
	}
	if email.subject != wantSubject {
		t.Errorf("subject: got %q, want %q", email.subject, wantSubject)
	}
	if strings.Contains(email.subject, "pruebas") || strings.Contains(email.rawSubject, "pruebas") {
		t.Errorf("subject must not contain 'pruebas'")
	}

	wantRawURL := mailer.actionURL(delivery.TokenID, delivery.Token)
	wantEscapedURL := html.EscapeString(wantRawURL)
	wantExpiry := "2026-10-27 18:30:00 UTC"

	// Assert text channel without dumping token/secrets on failure
	if email.textPart == "" {
		t.Fatal("missing text/plain part in multipart alternative")
	}
	if !strings.Contains(email.textPart, wantRawURL) {
		t.Errorf("text part missing required action URL")
	}
	if !strings.Contains(email.textPart, wantExpiry) {
		t.Errorf("text part missing readable UTC expiry")
	}
	if !strings.Contains(email.textPart, "EspaciGo") {
		t.Errorf("text part missing EspaciGo header")
	}
	if strings.Contains(email.textPart, "(pruebas)") {
		t.Errorf("text part must not contain '(pruebas)'")
	}

	// Honest copy in text part
	wantHonestCopy := "Si no solicitaste esta acción, puedes ignorar este mensaje. No compartas este enlace."
	if !strings.Contains(email.textPart, wantHonestCopy) {
		t.Errorf("text part missing honest ignore notice copy")
	}
	if strings.Contains(email.textPart, "tu información permanecerá protegida") ||
		strings.Contains(email.textPart, "tu clave actual sigue protegida") ||
		strings.Contains(email.textPart, "de forma segura") {
		t.Errorf("text part contains unverified safety promises")
	}

	// Assert HTML channel
	if email.htmlPart == "" {
		t.Fatal("missing text/html part in multipart alternative")
	}
	if !strings.Contains(email.htmlPart, wantEscapedURL) {
		t.Errorf("html part missing required action URL")
	}
	if !strings.Contains(email.htmlPart, wantExpiry) {
		t.Errorf("html part missing readable UTC expiry")
	}
	if !strings.Contains(email.htmlPart, "EspaciGo") {
		t.Errorf("html part missing typographic EspaciGo brand")
	}
	if strings.Contains(email.htmlPart, "<script") || strings.Contains(email.htmlPart, "http://") || (strings.Contains(email.htmlPart, "https://") && strings.Contains(email.htmlPart, "<img src=\"https://")) || strings.Contains(email.htmlPart, "data:image") {
		t.Errorf("html part must not have script tags or external images")
	}
	if !strings.Contains(email.htmlPart, "src=\"cid:espacigo-logo-") {
		t.Errorf("html part missing inline CID logo reference prefix src='cid:espacigo-logo-'")
	}
	if !strings.Contains(email.htmlPart, "alt=\"EspaciGo\"") {
		t.Errorf("html logo img missing alt='EspaciGo'")
	}
	if !strings.Contains(email.htmlPart, "width=\"40\"") || !strings.Contains(email.htmlPart, "height=\"40\"") {
		t.Errorf("html logo img missing width='40' height='40' dimensions")
	}
	if strings.Contains(email.htmlPart, "(pruebas)") {
		t.Errorf("html part must not contain '(pruebas)'")
	}

	// RFC 2387 & RFC 2392 assertions
	if email.relatedType != "multipart/alternative" {
		t.Errorf("expected related type parameter 'multipart/alternative', got %q", email.relatedType)
	}
	if email.rootMediaType != "multipart/alternative" {
		t.Errorf("expected root part media type 'multipart/alternative', got %q", email.rootMediaType)
	}
	if email.relatedType != email.rootMediaType {
		t.Errorf("RFC 2387 sec 3.1 violation: related type %q does not match root media type %q", email.relatedType, email.rootMediaType)
	}

	// Assert embedded logo attachment
	if !email.hasLogo {
		t.Fatal("missing embedded logo attachment in multipart/related")
	}
	if email.logoRawCID != "<"+email.logoCID+">" {
		t.Errorf("expected logo Content-ID enclosed in brackets, got %q", email.logoRawCID)
	}
	if !strings.HasPrefix(email.logoCID, "espacigo-logo-") || !strings.HasSuffix(email.logoCID, "@inline") {
		t.Errorf("expected logo CID to match espacigo-logo-*@inline, got %q", email.logoCID)
	}
	if _, err := mail.ParseAddress(email.logoCID); err != nil {
		t.Errorf("RFC 2392 violation: logo CID %q is not a valid addr-spec: %v", email.logoCID, err)
	}
	wantCIDAttr := fmt.Sprintf("src=\"cid:%s\"", email.logoCID)
	if !strings.Contains(email.htmlPart, wantCIDAttr) {
		t.Errorf("html part missing dynamic inline CID logo reference %s", wantCIDAttr)
	}
	if email.logoContentType != "image/png" {
		t.Errorf("expected logo Content-Type 'image/png', got %q", email.logoContentType)
	}
	if email.logoDisposition != "inline" {
		t.Errorf("expected logo Content-Disposition 'inline', got %q", email.logoDisposition)
	}
	if email.logoFilename != "espacigo-logo.png" {
		t.Errorf("expected logo filename 'espacigo-logo.png', got %q", email.logoFilename)
	}
	expectedLogoBytes, err := os.ReadFile("espacigo-logo.png")
	if err != nil {
		t.Fatalf("read fixture espacigo-logo.png: %v", err)
	}
	if !bytes.Equal(email.logoBytes, expectedLogoBytes) {
		t.Errorf("embedded logo bytes do not match espacigo-logo.png file (len got %d, want %d)", len(email.logoBytes), len(expectedLogoBytes))
	}

	// Honest copy in HTML part
	if !strings.Contains(email.htmlPart, wantHonestCopy) {
		t.Errorf("html part missing honest ignore notice copy")
	}
	if strings.Contains(email.htmlPart, "tu información permanecerá protegida") ||
		strings.Contains(email.htmlPart, "tu clave actual sigue protegida") ||
		strings.Contains(email.htmlPart, "de forma segura") {
		t.Errorf("html part contains unverified safety promises")
	}

	// Approved terracotta/cream palette colors check
	approvedColors := []string{"#A44731", "#FBF7F2", "#332720", "#66574E", "#DED5CC", "#FFFFFF", "#F5E3DB"}
	for _, col := range approvedColors {
		if !strings.Contains(email.htmlPart, col) {
			t.Errorf("html part missing approved palette color %s", col)
		}
	}

	// Ensure legacy orange/slate palette colors are NOT present
	unapprovedColors := []string{"#f97316", "#f8fafc", "#0f172a", "#475569", "#64748b", "#94a3b8", "#fff7ed", "#e2e8f0"}
	for _, col := range unapprovedColors {
		if strings.Contains(email.htmlPart, col) {
			t.Errorf("html part contains unapproved legacy color %s", col)
		}
	}
}

func TestSendRecoveryBrandedEmail(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")

	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo (pruebas)",
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}

	expiry := time.Date(2026, 10, 27, 19, 0, 0, 0, time.UTC)
	delivery := identity.RecoveryDelivery{
		AccountID: "acc-77",
		TokenID:   "recov-tok-12",
		Email:     "propietario@example.com",
		Token:     identity.Secret("rec-secret-456"),
		ExpiresAt: expiry,
	}

	err := mailer.SendRecovery(context.Background(), delivery)
	if err != nil {
		t.Fatalf("SendRecovery returned error: %v", err)
	}

	raw := srv.LastMessage()
	if raw == nil {
		t.Fatal("fake SMTP server received no message")
	}

	email := parseMultipartEmail(t, raw)

	// Assert From address header parses cleanly without 'pruebas' in Name
	fromAddr, err := mail.ParseAddress(email.from)
	if err != nil {
		t.Fatalf("parse From header error: %v", err)
	}
	if fromAddr.Name != "EspaciGo" {
		t.Errorf("expected From name 'EspaciGo', got %q", fromAddr.Name)
	}
	if fromAddr.Address != "contacto@hotusoft.com" {
		t.Errorf("expected From address 'contacto@hotusoft.com', got %q", fromAddr.Address)
	}

	// Assert RFC 2047 Q-encoded subject with accents (contiene 'ñ')
	if !strings.HasPrefix(email.rawSubject, "=?utf-8?q?") && !strings.HasPrefix(email.rawSubject, "=?UTF-8?Q?") && !strings.HasPrefix(email.rawSubject, "=?utf-8?Q?") {
		t.Errorf("subject is not RFC 2047 encoded: %q", email.rawSubject)
	}
	wantSubject := "Restablece tu contraseña de EspaciGo"
	if email.subject != wantSubject {
		t.Errorf("subject: got %q, want %q", email.subject, wantSubject)
	}
	if strings.Contains(email.subject, "pruebas") || strings.Contains(email.rawSubject, "pruebas") {
		t.Errorf("subject must not contain 'pruebas'")
	}

	wantRawURL := mailer.config.PublicOrigin + "/recuperar-acceso#" + "token=rec-secret-456&token_id=recov-tok-12"
	wantEscapedURL := html.EscapeString(wantRawURL)
	wantExpiry := "2026-10-27 19:00:00 UTC"

	if !strings.Contains(email.textPart, wantRawURL) || !strings.Contains(email.textPart, wantExpiry) {
		t.Errorf("text part missing url or expiry")
	}
	if !strings.Contains(email.htmlPart, wantEscapedURL) || !strings.Contains(email.htmlPart, wantExpiry) {
		t.Errorf("html part missing url or expiry")
	}

	wantHonestCopy := "Si no solicitaste esta acción, puedes ignorar este mensaje. No compartas este enlace."
	if !strings.Contains(email.textPart, wantHonestCopy) {
		t.Errorf("text part missing honest ignore notice copy")
	}
	if strings.Contains(email.textPart, "tu información permanecerá protegida") ||
		strings.Contains(email.textPart, "tu clave actual sigue protegida") ||
		strings.Contains(email.textPart, "de forma segura") {
		t.Errorf("text part contains unverified safety promises")
	}

	if !strings.Contains(email.htmlPart, wantHonestCopy) {
		t.Errorf("html part missing honest ignore notice copy")
	}
	if strings.Contains(email.htmlPart, "tu información permanecerá protegida") ||
		strings.Contains(email.htmlPart, "tu clave actual sigue protegida") ||
		strings.Contains(email.htmlPart, "de forma segura") {
		t.Errorf("html part contains unverified safety promises")
	}

	if !email.hasLogo {
		t.Fatal("SendRecovery email missing embedded logo in multipart/related")
	}
	if !strings.Contains(email.htmlPart, "src=\"cid:"+email.logoCID+"\"") {
		t.Errorf("SendRecovery HTML missing src='cid:%s'", email.logoCID)
	}
}

func TestSendPasswordChangedAndLoginAlertSubjects(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")

	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo (pruebas)",
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}

	dec := new(mime.WordDecoder)

	// Test SendPasswordChanged (contains accent 'ó')
	if err := mailer.SendPasswordChanged(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("SendPasswordChanged error: %v", err)
	}
	raw := srv.LastMessage()
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("read password changed message: %v", err)
	}
	fromAddr, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		t.Fatalf("parse password changed From: %v", err)
	}
	if fromAddr.Name != "EspaciGo" {
		t.Errorf("expected From name 'EspaciGo', got %q", fromAddr.Name)
	}
	rawSub := msg.Header.Get("Subject")
	if !strings.HasPrefix(rawSub, "=?utf-8?q?") && !strings.HasPrefix(rawSub, "=?UTF-8?Q?") && !strings.HasPrefix(rawSub, "=?utf-8?Q?") {
		t.Errorf("password changed subject is not RFC 2047 encoded: %q", rawSub)
	}
	sub, err := dec.DecodeHeader(rawSub)
	if err != nil {
		t.Fatalf("decode password changed subject error: %v", err)
	}
	if sub != "Actualización de clave de EspaciGo" {
		t.Errorf("password changed subject: got %q, want 'Actualización de clave de EspaciGo'", sub)
	}
	if strings.Contains(sub, "pruebas") {
		t.Errorf("subject must not contain 'pruebas': %q", sub)
	}

	// Test SendLoginAlert (ASCII)
	until := time.Date(2026, 10, 27, 20, 0, 0, 0, time.UTC)
	if err := mailer.SendLoginAlert(context.Background(), "user@example.com", until); err != nil {
		t.Fatalf("SendLoginAlert error: %v", err)
	}
	rawAlert := srv.LastMessage()
	msgAlert, err := mail.ReadMessage(bytes.NewReader(rawAlert))
	if err != nil {
		t.Fatalf("read alert message: %v", err)
	}
	fromAlertAddr, err := mail.ParseAddress(msgAlert.Header.Get("From"))
	if err != nil {
		t.Fatalf("parse alert From: %v", err)
	}
	if fromAlertAddr.Name != "EspaciGo" {
		t.Errorf("expected From name 'EspaciGo', got %q", fromAlertAddr.Name)
	}
	rawAlertSub := msgAlert.Header.Get("Subject")
	wantAlertSub := "Aviso de acceso a EspaciGo"
	if rawAlertSub != mime.QEncoding.Encode("utf-8", wantAlertSub) {
		t.Errorf("alert subject was not Q-encoded: got %q, want %q", rawAlertSub, mime.QEncoding.Encode("utf-8", wantAlertSub))
	}
	subAlert, err := dec.DecodeHeader(rawAlertSub)
	if err != nil {
		t.Fatalf("decode alert subject error: %v", err)
	}
	if subAlert != wantAlertSub {
		t.Errorf("login alert subject: got %q, want %q", subAlert, wantAlertSub)
	}
	if strings.Contains(subAlert, "pruebas") {
		t.Errorf("alert subject must not contain 'pruebas': %q", subAlert)
	}
}

func TestSendLocalBookingNoticePreserved(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")
	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo (pruebas)",
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}
	if err := mailer.SendLocalBookingNotice(context.Background(), "user@example.com", "Reserva confirmada", "Detalles de la reserva"); err != nil {
		t.Fatalf("SendLocalBookingNotice error: %v", err)
	}
	raw := srv.LastMessage()
	msg, _ := mail.ReadMessage(bytes.NewReader(raw))
	fromAddr, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		t.Fatalf("parse booking notice From: %v", err)
	}
	if fromAddr.Name != "EspaciGo (pruebas)" {
		t.Errorf("expected booking notice From name 'EspaciGo (pruebas)', got %q", fromAddr.Name)
	}
	if fromAddr.Address != "contacto@hotusoft.com" {
		t.Errorf("expected booking notice From address 'contacto@hotusoft.com', got %q", fromAddr.Address)
	}
	dec := new(mime.WordDecoder)
	sub, _ := dec.DecodeHeader(msg.Header.Get("Subject"))
	if !strings.Contains(sub, "Reserva confirmada (EspaciGo pruebas)") {
		t.Errorf("expected booking notice subject to contain 'Reserva confirmada (EspaciGo pruebas)', got %q", sub)
	}
	bookingCT := msg.Header.Get("Content-Type")
	if !strings.HasPrefix(bookingCT, "text/plain") || strings.Contains(bookingCT, "multipart") || strings.Contains(bookingCT, "related") {
		t.Errorf("expected booking notice to be plain text without attachments/multipart, got %q", bookingCT)
	}
}

func TestCustomFromNamePreservedForAuth(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")
	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo Custom Brand",
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}

	delivery := identity.VerificationDelivery{
		AccountID: "acc-custom",
		TokenID:   "verif-custom",
		Email:     "custom@example.com",
		Token:     identity.Secret("sec-tok"),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := mailer.SendVerification(context.Background(), delivery); err != nil {
		t.Fatalf("SendVerification error: %v", err)
	}
	raw := srv.LastMessage()
	msg, _ := mail.ReadMessage(bytes.NewReader(raw))
	fromAddr, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		t.Fatalf("parse custom From: %v", err)
	}
	if fromAddr.Name != "EspaciGo Custom Brand" {
		t.Errorf("expected custom From name 'EspaciGo Custom Brand', got %q", fromAddr.Name)
	}
	if fromAddr.Address != "contacto@hotusoft.com" {
		t.Errorf("expected From address 'contacto@hotusoft.com', got %q", fromAddr.Address)
	}
}

func TestAuthHTMLUsesValidPercentageWidths(t *testing.T) {
	body := renderAuthEmailHTML(authEmailData{Title: "EspaciGo", ButtonURL: "#"})
	if strings.Contains(body, `width="100%%"`) || !strings.Contains(body, `width="100%"`) {
		t.Fatal("HTML table widths must contain a single percentage sign")
	}
}

func TestSubjectRejectsCRLFInjection(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")
	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo",
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}

	injectionSubjects := []string{
		"Subject\r\nBcc: hacker@example.com",
		"Subject\nInjected: true",
		"Subject\rInjected: true",
	}
	for _, sub := range injectionSubjects {
		err := mailer.SendLocalBookingNotice(context.Background(), "user@example.com", sub, "body")
		if err == nil || !errors.Is(err, identity.ErrDelivery) {
			t.Errorf("expected ErrDelivery for CRLF injection in subject %q, got: %v", sub, err)
		}
	}
}

func TestRFCInteroperabilityRelatedRootTypeAndUniqueContentIDAddrSpec(t *testing.T) {
	srv := startFakeSMTPServer(t, "contacto@hotusoft.com", "secret123")
	mailer := &Mailer{
		config: Config{
			Host:         "smtp.zoho.com",
			Port:         "587",
			Security:     "starttls",
			Username:     "contacto@hotusoft.com",
			From:         "contacto@hotusoft.com",
			FromName:     "EspaciGo",
			PublicOrigin: "https://espacigo.testingpage.store",
		},
		password:  "secret123",
		tlsConfig: srv.clientTLS,
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("tcp", srv.listener.Addr().String())
		},
	}

	del1 := identity.VerificationDelivery{
		AccountID: "acc-1",
		TokenID:   "tok-1",
		Email:     "u1@example.com",
		Token:     identity.Secret("sec-1"),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := mailer.SendVerification(context.Background(), del1); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	raw1 := srv.LastMessage()
	msg1 := parseMultipartEmail(t, raw1)

	del2 := identity.VerificationDelivery{
		AccountID: "acc-2",
		TokenID:   "tok-2",
		Email:     "u2@example.com",
		Token:     identity.Secret("sec-2"),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := mailer.SendVerification(context.Background(), del2); err != nil {
		t.Fatalf("send 2: %v", err)
	}
	raw2 := srv.LastMessage()
	msg2 := parseMultipartEmail(t, raw2)

	// RFC 2387 Sec 3.1: outer related type parameter MUST match root part media type
	for idx, msg := range []parsedEmail{msg1, msg2} {
		if msg.relatedType != "multipart/alternative" {
			t.Errorf("msg %d: expected related type parameter 'multipart/alternative', got %q", idx+1, msg.relatedType)
		}
		if msg.rootMediaType != "multipart/alternative" {
			t.Errorf("msg %d: expected root part media type 'multipart/alternative', got %q", idx+1, msg.rootMediaType)
		}
		if msg.relatedType != msg.rootMediaType {
			t.Errorf("msg %d: RFC 2387 mismatch: related type %q != root media type %q", idx+1, msg.relatedType, msg.rootMediaType)
		}

		// RFC 2392 Sec 2: Content-ID MUST be a valid addr-spec
		if msg.logoRawCID != "<"+msg.logoCID+">" {
			t.Errorf("msg %d: expected Content-ID in brackets: %q", idx+1, msg.logoRawCID)
		}
		if !strings.Contains(msg.logoCID, "@") {
			t.Errorf("msg %d: Content-ID %q missing '@' for RFC 2392 addr-spec", idx+1, msg.logoCID)
		}
		if _, err := mail.ParseAddress(msg.logoCID); err != nil {
			t.Errorf("msg %d: invalid addr-spec in Content-ID %q: %v", idx+1, msg.logoCID, err)
		}

		// HTML resolves the same CID as the attached PNG asset
		wantSrc := fmt.Sprintf("src=\"cid:%s\"", msg.logoCID)
		if !strings.Contains(msg.htmlPart, wantSrc) {
			t.Errorf("msg %d: HTML does not resolve attached logo CID %s", idx+1, wantSrc)
		}
	}

	// Globally unique CIDs across distinct messages
	if msg1.logoCID == msg2.logoCID {
		t.Errorf("expected distinct globally unique CIDs, but got identical %q", msg1.logoCID)
	}
}
