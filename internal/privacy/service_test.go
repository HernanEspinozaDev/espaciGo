package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryRepository struct {
	profile  Profile
	requests []RightsRequest
}

func (m *memoryRepository) GetProfile(context.Context, string) (Profile, error) {
	if m.profile.AccountID == "" {
		return Profile{}, ErrNotFound
	}
	return m.profile, nil
}
func (m *memoryRepository) SaveProfile(_ context.Context, id, name string, phone *string) (Profile, error) {
	m.profile = Profile{AccountID: id, Name: name, Phone: phone, UpdatedAt: time.Unix(1, 0)}
	return m.profile, nil
}
func (m *memoryRepository) CreateRightsRequest(_ context.Context, id, kind, channel string) (RightsRequest, error) {
	r := RightsRequest{ID: "synthetic-id", Kind: kind, State: "en_revision", CreatedAt: time.Unix(1, 0)}
	m.requests = append(m.requests, r)
	return r, nil
}
func (m *memoryRepository) ListOwnRightsRequests(context.Context, string) ([]RightsRequest, error) {
	return m.requests, nil
}
func (m *memoryRepository) ExportOwnData(_ context.Context, accountID string) (OwnData, error) {
	return OwnData{Account: ExportAccount{Email: accountID, State: "activo"}, Roles: []string{"arrendatario"}, Acceptances: []TermsAcceptance{}, Requests: []RightsRequest{}, Scope: "identidad_local_v1"}, nil
}

func TestProfileValidationAndRightsRemainPending(t *testing.T) {
	repo := &memoryRepository{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Profile(context.Background(), "account"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing profile err=%v", err)
	}
	if _, err := svc.UpdateProfile(context.Background(), "account", "   ", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty name err=%v", err)
	}
	if _, err := svc.UpdateProfile(context.Background(), "account", "Synthetic 42", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nonalphabetic name err=%v", err)
	}
	if _, err := svc.UpdateProfile(context.Background(), "account", "Ada Ejemplo", "12345"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad phone err=%v", err)
	}
	profile, err := svc.UpdateProfile(context.Background(), "account", " Ada Ejemplo ", "123456789")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "Ada Ejemplo" || profile.Phone == nil || *profile.Phone != "123456789" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	request, err := svc.RequestRight(context.Background(), "account", "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	if request.State != "en_revision" {
		t.Fatalf("request state=%s", request.State)
	}
	if _, err := svc.RequestRight(context.Background(), "account", "delete_everything", "web"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsupported request err=%v", err)
	}
}

func TestOwnDataExportHasBoundedScopeAndNoCredentialFields(t *testing.T) {
	repo := &memoryRepository{}
	svc, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ExportOwnData(context.Background(), ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty account export err=%v", err)
	}
	data, err := svc.ExportOwnData(context.Background(), "own@example.invalid")
	if err != nil || data.Account.Email != "own@example.invalid" || data.Scope != "identidad_local_v1" {
		t.Fatalf("export=%+v err=%v", data, err)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password_hash", "token", "session", "secret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("export contains forbidden field %q: %s", forbidden, encoded)
		}
	}
}
