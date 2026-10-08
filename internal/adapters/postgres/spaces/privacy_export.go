package spacespg

import (
	"context"
	"encoding/json"
)

// ExportOwnArchiveSections delegates draft selection to the same owner-scoped
// repository contract used by the spaces module's authenticated API.
func (r *Repository) ExportOwnArchiveSections(ctx context.Context, owner string) (map[string]json.RawMessage, error) {
	drafts, err := r.ListOwn(ctx, owner)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(drafts)
	if err != nil {
		return nil, err
	}
	return map[string]json.RawMessage{"spaces": encoded}, nil
}

var _ interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
} = (*Repository)(nil)
