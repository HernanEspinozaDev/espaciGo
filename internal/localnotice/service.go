// Package localnotice delivers the transactional synthetic notice intents via Mailpit.
package localnotice

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

var ErrInvalid = errors.New("localnotice: invalid request")

type Notice struct {
	ID, Type, RecipientID, Email, AggregateType, AggregateID string
	Cycle, Attempt                                           int
}
type TerminalNotice struct {
	ID         string    `json:"event_id"`
	Type       string    `json:"event_type"`
	State      string    `json:"state"`
	Code       string    `json:"error_code"`
	Cycle      int       `json:"cycle_number"`
	Attempts   int       `json:"total_attempts"`
	CreatedAt  time.Time `json:"created_at"`
	TerminalAt time.Time `json:"terminal_at"`
}
type Recovery struct {
	NoticeID string `json:"notice_id"`
	Cycle    int    `json:"cycle"`
	Reused   bool   `json:"reused,omitempty"`
}
type Repository interface {
	Claim(context.Context, time.Time) (Notice, bool, error)
	Finish(context.Context, Notice, bool, string, time.Time) error
	ListTerminal(context.Context, int) ([]TerminalNotice, error)
	Reopen(context.Context, string, string, string, string, string, time.Time) (Recovery, error)
	Purge(context.Context, time.Time, int) (int64, error)
}
type Sender interface {
	SendLocalBookingNotice(context.Context, string, string, string) error
}
type Service struct {
	repo           Repository
	sender         Sender
	now            func() time.Time
	frontendOrigin string
}

func New(repo Repository, sender Sender, now func() time.Time) (*Service, error) {
	return NewWithFrontendOrigin(repo, sender, now, "http://127.0.0.1:4200")
}

func NewWithFrontendOrigin(repo Repository, sender Sender, now func() time.Time, frontendOrigin string) (*Service, error) {
	if repo == nil || sender == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	parsed, err := url.Parse(frontendOrigin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Path != "" && parsed.Path != "/") {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, sender: sender, now: now, frontendOrigin: strings.TrimRight(frontendOrigin, "/")}, nil
}
func (s *Service) DispatchOne(ctx context.Context) (bool, error) {
	n, ok, e := s.repo.Claim(ctx, s.now().UTC())
	if e != nil || !ok {
		return ok, e
	}
	subject, body := renderWithFrontendOrigin(n, s.frontendOrigin)
	sendErr := s.sender.SendLocalBookingNotice(ctx, n.Email, subject, body)
	code := ""
	if sendErr != nil {
		code = "mailpit_delivery_failed"
	}
	if e = s.repo.Finish(ctx, n, sendErr == nil, code, s.now().UTC()); e != nil {
		return true, e
	}
	return true, nil
}
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if interval < 250*time.Millisecond {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.DispatchOne(ctx)
		}
	}
}
func (s *Service) Terminal(ctx context.Context, limit int) ([]TerminalNotice, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	return s.repo.ListTerminal(ctx, limit)
}
func (s *Service) Reopen(ctx context.Context, id, admin, reason, key, correlation string) (Recovery, error) {
	if id == "" || admin == "" || key == "" || correlation == "" {
		return Recovery{}, ErrInvalid
	}
	switch reason {
	case "smtp_restaurado", "reintento_operativo":
	default:
		return Recovery{}, ErrInvalid
	}
	return s.repo.Reopen(ctx, id, admin, reason, key, correlation, s.now().UTC())
}
func (s *Service) Purge(ctx context.Context, limit int) (int64, error) {
	if limit < 1 || limit > 500 {
		return 0, ErrInvalid
	}
	return s.repo.Purge(ctx, s.now().UTC(), limit)
}
func render(n Notice) (string, string) {
	return renderWithFrontendOrigin(n, "http://127.0.0.1:4200")
}

func renderWithFrontendOrigin(n Notice, frontendOrigin string) (string, string) {
	switch n.Type {
	case "cuenta_bloqueada_login":
		return "Bloqueo temporal de inicio de sesión (ensayo local)", "Se bloqueó temporalmente el inicio de sesión por intentos fallidos. Si no reconoces la actividad, cambia la contraseña. Este aviso no incluye credenciales ni tokens."
	case "verificacion_aprobada":
		return "Verificación sintética aprobada", "Tu caso de verificación local fue aprobado. Inicia sesión en el prototipo para consultar su estado y elegibilidad vigente."
	case "verificacion_rechazada":
		return "Verificación sintética rechazada", "Tu caso de verificación local fue rechazado. Inicia sesión en el prototipo para consultar el motivo estructurado y los siguientes pasos."
	case "pago_arriendo_confirmado":
		return "Pago simulado confirmado", "El pago simulado del arriendo fue confirmado. Inicia sesión para consultar el estado vigente de la reserva. ENSAYO LOCAL — SIN COBRO REAL."
	case "pago_arriendo_rechazado":
		return "Pago simulado rechazado", "El pago simulado del arriendo fue rechazado. Inicia sesión para consultar el estado vigente de la reserva. ENSAYO LOCAL — SIN COBRO REAL."
	case "nueva_reserva_pagada":
		return "Nueva solicitud pagada (ensayo local)", "Una solicitud sintética quedó pagada y espera tu decisión como anfitrión. Inicia sesión para revisar la reserva. ENSAYO LOCAL — SIN COBRO REAL."
	case "invitacion_firma":
		return "Invitación a firmar contrato sintético", "Tienes un contrato sintético pendiente. Inicia sesión y abre la reserva indicada para revisar el documento: " + contractLink(frontendOrigin, n.AggregateID) + "\nEl enlace no autentica ni concede acceso; el Backend verifica tu participación. ENSAYO SINTÉTICO LOCAL — SIN VALIDEZ JURÍDICA."
	case "contrato_firmado":
		return "Contrato sintético firmado por ambas partes", "Ambas firmas sintéticas están completas. Inicia sesión en el prototipo para revisar la reserva y su historial. ENSAYO SINTÉTICO LOCAL — SIN VALIDEZ JURÍDICA."
	case "checkin_registrado":
		return "Registro de check-in (ensayo local)", "Se registró un check-in sintético en una reserva. Inicia sesión en el prototipo local para revisar el estado."
	case "resena_reportada":
		return "Reseña reportada (ensayo local)", "Una reseña sintética requiere revisión administrativa. Inicia sesión en el prototipo local para revisar la cola."
	case "reclamo_abierto":
		return "Reclamo abierto (ensayo local)", "Se abrió un reclamo sintético asociado a una reserva. Inicia sesión en el prototipo local para revisar el estado."
	case "reclamo_resuelto":
		return "Resolución de reclamo (ensayo local)", "Se registró una resolución sintética de un reclamo. Inicia sesión en el prototipo local para revisar el resultado. No se movieron fondos."
	default:
		return "Actualización de reserva (ensayo local)", "Se actualizó una reserva sintética. Inicia sesión en el prototipo local para revisar su estado."
	}
}

func contractLink(origin, id string) string {
	if len(id) != 36 {
		return origin + "/reservas"
	}
	return origin + "/reservas?contrato=" + url.QueryEscape(id)
}
