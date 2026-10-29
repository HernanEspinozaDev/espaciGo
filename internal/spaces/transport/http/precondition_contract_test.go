package spaceshttp

import (
	"net/http"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

func TestPutDraftWithoutIfMatchRequiresPrecondition(t *testing.T) {
	repo := &memoryRepo{
		owner: ownerA,
		draft: draftFromInput(spaces.Input{
			CategoryCode: "oficina",
			Title:        "Original draft title",
			Description:  "Original draft description long enough to be valid.",
			AreaM2:       10,
			Capacity:     3,
			UsageRules:   "Sin fumar",
			RateUnit:     "dia",
			BasePriceCLP: 6001,
			Address:      "Calle 1",
		}),
	}
	repo.draft.ID = draftID
	repo.draft.State = "borrador"

	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, svc, nil)

	body := strings.Replace(validJSON(), `"category_code":`, `"attribute_schema_version":1,"category_code":`, 1)
	response := invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID, "landlord", body)
	if response.Code != http.StatusPreconditionRequired {
		t.Errorf("PUT without If-Match status=%d want %d; body=%s", response.Code, http.StatusPreconditionRequired, response.Body.String())
	}
	if repo.draft.Title != "Original draft title" {
		t.Errorf("draft mutated without If-Match: title=%q", repo.draft.Title)
	}
}
