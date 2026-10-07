package identity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/cases"
)

// AccountState is an account lifecycle state, never an HTTP status.
type AccountState string

const (
	AccountEmailPending     AccountState = "correo_pendiente"
	AccountActive           AccountState = "activo"
	AccountBlocked          AccountState = "bloqueado"
	AccountClosureRequested AccountState = "baja_solicitada"
	AccountDeidentified     AccountState = "desidentificado"
)

func (s AccountState) Valid() bool {
	switch s {
	case AccountEmailPending, AccountActive, AccountBlocked, AccountClosureRequested, AccountDeidentified:
		return true
	}
	return false
}

type Role string

const (
	RoleTenant        Role = "arrendatario"
	RoleLandlord      Role = "arrendador"
	RoleAdministrator Role = "administrador"
)

// Role is deliberately explicit: registration must never grant administrator privileges.
func RegistrationRoles() []Role { return []Role{RoleTenant} }

type Account struct {
	ID              string
	Email           string
	NormalizedEmail string
	PasswordHash    Secret
	UsePreference   string
	State           AccountState
	CreatedAt       time.Time
	UpdatedAt       time.Time
	FailedAttempts  int
	BlockedUntil    *time.Time
}

// Secret prevents accidental disclosure in formatting and structured logs.
type Secret string

func (Secret) String() string               { return "[REDACTED]" }
func (Secret) GoString() string             { return "[REDACTED]" }
func (Secret) MarshalText() ([]byte, error) { return []byte("[REDACTED]"), nil }
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func NormalizeEmail(email string) string { return cases.Fold().String(strings.TrimSpace(email)) }

var (
	ErrNotFound = errors.New("identity: not found")
	ErrConflict = errors.New("identity: conflict")
	ErrInvalid  = errors.New("identity: invalid value")
)

func (a Account) Validate() error {
	if a.ID == "" || a.Email == "" || a.NormalizedEmail == "" || a.NormalizedEmail != NormalizeEmail(a.Email) || a.PasswordHash == "" || !a.State.Valid() || a.FailedAttempts < 0 || (a.UsePreference != "" && a.UsePreference != "ofrecer" && a.UsePreference != "arrendar") {
		return fmt.Errorf("%w: account", ErrInvalid)
	}
	return nil
}

type Session struct {
	ID, AccountID                        string
	TokenHash                            string `json:"-"`
	CreatedAt, LastActivityAt, ExpiresAt time.Time
	RevokedAt                            *time.Time
	ClientSummary                        string
}

func (Session) String() string   { return "Session{TokenHash:[REDACTED]}" }
func (Session) GoString() string { return "Session{TokenHash:[REDACTED]}" }

func (s Session) ActiveAt(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt) && now.Before(s.LastActivityAt.Add(30*time.Minute))
}

type TermsVersion struct {
	ID, Code, Type, SHA256 string
	PublishedAt            time.Time
}

type TermsAcceptance struct {
	ID, AccountID, VersionID, Channel string
	AcceptedAt                        time.Time
}

type ActionToken struct {
	ID, AccountID, Purpose    string
	Hash                      string `json:"-"`
	CreatedAt, ExpiresAt      time.Time
	ConsumedAt, InvalidatedAt *time.Time
	Attempts                  int
}

func (ActionToken) String() string   { return "ActionToken{Hash:[REDACTED]}" }
func (ActionToken) GoString() string { return "ActionToken{Hash:[REDACTED]}" }
