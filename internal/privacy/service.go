package privacy

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	ErrInvalid  = errors.New("invalid privacy input")
	ErrNotFound = errors.New("privacy resource not found")
)

type Profile struct {
	AccountID string
	Name      string
	Phone     *string
	UpdatedAt time.Time
}

type RightsRequest struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository interface {
	GetProfile(context.Context, string) (Profile, error)
	SaveProfile(context.Context, string, string, *string) (Profile, error)
	CreateRightsRequest(context.Context, string, string, string) (RightsRequest, error)
	ListOwnRightsRequests(context.Context, string) ([]RightsRequest, error)
}

type Service struct{ repo Repository }

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo}, nil
}

func (s *Service) Profile(ctx context.Context, accountID string) (Profile, error) {
	if accountID == "" {
		return Profile{}, ErrInvalid
	}
	return s.repo.GetProfile(ctx, accountID)
}

var nineDigits = regexp.MustCompile(`^[0-9]{9}$`)

func (s *Service) UpdateProfile(ctx context.Context, accountID, name, phone string) (Profile, error) {
	name = strings.TrimSpace(name)
	validName := name != "" && len([]rune(name)) <= 120
	for _, character := range name {
		validName = validName && (unicode.IsLetter(character) || character == ' ')
	}
	if accountID == "" || !validName {
		return Profile{}, ErrInvalid
	}
	var normalized *string
	if phone != "" {
		if !nineDigits.MatchString(phone) {
			return Profile{}, ErrInvalid
		}
		normalized = &phone
	}
	return s.repo.SaveProfile(ctx, accountID, name, normalized)
}

func (s *Service) RequestRight(ctx context.Context, accountID, kind, channel string) (RightsRequest, error) {
	if accountID == "" || (kind != "acceso" && kind != "supresion") || (channel != "web" && channel != "api") {
		return RightsRequest{}, ErrInvalid
	}
	return s.repo.CreateRightsRequest(ctx, accountID, kind, channel)
}

func (s *Service) OwnRequests(ctx context.Context, accountID string) ([]RightsRequest, error) {
	if accountID == "" {
		return nil, ErrInvalid
	}
	return s.repo.ListOwnRightsRequests(ctx, accountID)
}
