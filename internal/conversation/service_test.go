package conversation

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
)

type serviceRepo struct {
	sent   int
	cursor int64
}

func (r *serviceRepo) List(context.Context, string, string, *int64, int) (Page, error) {
	return Page{Items: []Message{}}, nil
}
func (r *serviceRepo) Send(_ context.Context, actor, reservation, key, body string, _ []byte, id string, now func() time.Time) (Message, error) {
	r.sent++
	return Message{ID: id, ReservationID: reservation, AuthorID: actor, Body: body, CreatedAt: now()}, nil
}
func (r *serviceRepo) MarkRead(_ context.Context, _, _ string, through int64) (int64, error) {
	if through < 1 {
		return 0, ErrInvalid
	}
	if through > r.cursor {
		r.cursor = through
	}
	return r.cursor, nil
}

func TestServiceValidatesMessageTextAndPageBounds(t *testing.T) {
	repo := &serviceRepo{}
	service, err := NewService(repo, credentials.Generator{}, func() time.Time { return time.Unix(100, 0) })
	if err != nil {
		t.Fatal(err)
	}
	actor := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	reservation := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	for _, body := range []string{"", " \n ", strings.Repeat("x", MaxMessageRunes+1)} {
		if _, err = service.Send(context.Background(), actor, reservation, "key", body); err != ErrInvalid {
			t.Fatalf("invalid body length=%d accepted: %v", utf8.RuneCountInString(body), err)
		}
	}
	body := strings.Repeat("á🙂", MaxMessageRunes/2)
	if _, err = service.Send(context.Background(), actor, reservation, "key", body); err != nil || repo.sent != 1 {
		t.Fatalf("2000 Unicode characters rejected or not stored: sent=%d err=%v", repo.sent, err)
	}
	if _, err = service.List(context.Background(), actor, reservation, nil, MaxPageSize+1); err != ErrInvalid {
		t.Fatalf("oversized page accepted: %v", err)
	}
	for _, through := range []int64{12, 8, 12, 19} {
		cursor, markErr := service.MarkRead(context.Background(), actor, reservation, through)
		if markErr != nil || cursor != maxInt64(through, 12) {
			t.Fatalf("mark read through=%d returned cursor=%d err=%v", through, cursor, markErr)
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
