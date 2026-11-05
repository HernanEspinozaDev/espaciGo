package spaceshttp

import (
	"context"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func (m *memoryRepo) UpdatePublishedOwnIfRevision(ctx context.Context, owner, id string, expected time.Time, in spaces.PublishedContentInput) (spaces.Draft, error) {
	if owner != m.owner || id != m.draft.ID {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	if !expected.Equal(m.draft.UpdatedAt) {
		return spaces.Draft{}, spaces.ErrPreconditionFailed
	}
	item, err := m.UpdatePublishedOwn(ctx, owner, id, in)
	if err == nil {
		m.draft.UpdatedAt = expected.Add(time.Microsecond)
		item = m.draft
	}
	return item, err
}
func TestPublishedContentPreconditions(t *testing.T) {
	for _, tc := range []struct {
		name, tag, body string
		status          int
	}{
		{"missing-before-body", "", "{", 428},
		{"malformed", "*", `{"title":"Next"}`, 400},
		{"wrong-resource", draftETag(spaces.Draft{ID: ownerB, UpdatedAt: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}), `{"title":"Next"}`, 412},
		{"stale", draftETag(spaces.Draft{ID: draftID, UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}), `{"title":"Next"}`, 412},
		{"current", "current", `{"title":"Next"}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
			repo := &memoryRepo{owner: ownerA, draft: spaces.Draft{ID: draftID, State: "activa", Title: "Original", UpdatedAt: now}}
			svc, _ := spaces.NewService(repo)
			h := NewHandler(fakeAuth{}, svc, nil)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+draftID+"/publication-content", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer landlord")
			req.Header.Set("Content-Type", "application/json")
			if tc.tag == "current" {
				req.Header.Set("If-Match", draftETag(repo.draft))
			} else if tc.tag != "" {
				req.Header.Set("If-Match", tc.tag)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if tc.status != 200 && (repo.draft.Title != "Original" || !repo.draft.UpdatedAt.Equal(now)) {
				t.Fatal("mutation on rejected request")
			}
			if tc.status == 200 && (w.Header().Get("ETag") != draftETag(repo.draft) || !repo.draft.UpdatedAt.After(now)) {
				t.Fatal("missing renewed ETag")
			}
		})
	}
}
