// Package localnotice delivers the transactional synthetic notice intents via Mailpit.
package localnotice

import (
	"context"
	"errors"
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
	repo   Repository
	sender Sender
	now    func() time.Time
}

func New(repo Repository, sender Sender, now func() time.Time) (*Service, error) {
	if repo == nil || sender == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, sender: sender, now: now}, nil
}
func (s *Service) DispatchOne(ctx context.Context) (bool, error) {
	n, ok, e := s.repo.Claim(ctx, s.now().UTC())
	if e != nil || !ok {
		return ok, e
	}
	subject, body := render(n)
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
	switch n.Type {
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
