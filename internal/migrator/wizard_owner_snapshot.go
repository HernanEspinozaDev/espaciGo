package migrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type wizardObjectKey struct {
	catalog string
	oid     uint32
}

type wizardOwnerState struct {
	owner           uint32
	parentTable     uint32
	securityDefiner bool
}

type wizardOwnerSnapshot map[wizardObjectKey]wizardOwnerState

type wizardOwnerQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func captureWizardOwnerSnapshot(ctx context.Context, q wizardOwnerQueryer) (wizardOwnerSnapshot, error) {
	const query = `
SELECT 'pg_class'::text, c.oid::bigint, c.relowner::bigint,
       COALESCE(i.indrelid, 0)::bigint, false
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_index i ON i.indexrelid = c.oid AND c.relkind = 'i'
WHERE n.nspname = 'public'
UNION ALL
SELECT 'pg_proc'::text, p.oid::bigint, p.proowner::bigint, 0::bigint, p.prosecdef
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = 'public'`
	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query wizard ownership snapshot: %w", err)
	}
	defer rows.Close()
	snapshot := make(wizardOwnerSnapshot)
	for rows.Next() {
		var catalog string
		var oid, owner, parent int64
		var definer bool
		if err := rows.Scan(&catalog, &oid, &owner, &parent, &definer); err != nil {
			return nil, fmt.Errorf("scan wizard ownership snapshot: %w", err)
		}
		if oid < 0 || uint64(oid) > uint64(^uint32(0)) || owner < 0 || uint64(owner) > uint64(^uint32(0)) || parent < 0 || uint64(parent) > uint64(^uint32(0)) || (catalog != "pg_class" && catalog != "pg_proc") {
			return nil, errors.New("invalid identity in wizard ownership snapshot")
		}
		key := wizardObjectKey{catalog: catalog, oid: uint32(oid)}
		if _, exists := snapshot[key]; exists {
			return nil, fmt.Errorf("duplicate object in wizard ownership snapshot: %s/%d", catalog, oid)
		}
		snapshot[key] = wizardOwnerState{owner: uint32(owner), parentTable: uint32(parent), securityDefiner: definer}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wizard ownership snapshot: %w", err)
	}
	return snapshot, nil
}

func verifyWizardOwnerSnapshot(before, after wizardOwnerSnapshot, expected map[wizardObjectKey]bool, schemaOwner uint32) error {
	for key, prior := range before {
		current, ok := after[key]
		if !ok {
			return fmt.Errorf("preexisting object disappeared: %s/%d", key.catalog, key.oid)
		}
		if current.owner != prior.owner {
			return fmt.Errorf("preexisting object owner changed: %s/%d", key.catalog, key.oid)
		}
		if key.catalog == "pg_proc" && current.securityDefiner != prior.securityDefiner {
			return fmt.Errorf("preexisting function security-definer flag changed: %d", key.oid)
		}
	}
	for key, wantDefiner := range expected {
		if _, existed := before[key]; existed {
			return fmt.Errorf("expected object already existed: %s/%d", key.catalog, key.oid)
		}
		got, ok := after[key]
		if !ok {
			return fmt.Errorf("expected object is missing: %s/%d", key.catalog, key.oid)
		}
		if got.owner != schemaOwner {
			return fmt.Errorf("new object has wrong owner: %s/%d", key.catalog, key.oid)
		}
		if key.catalog == "pg_proc" && got.securityDefiner != wantDefiner {
			return fmt.Errorf("new function has wrong security-definer flag: %d", key.oid)
		}
	}
	for key, got := range after {
		if _, existed := before[key]; existed {
			continue
		}
		if got.owner != schemaOwner {
			return fmt.Errorf("new object has wrong owner: %s/%d", key.catalog, key.oid)
		}
		if _, listed := expected[key]; listed {
			continue
		}
		if key.catalog == "pg_class" && got.parentTable != 0 {
			if _, parentAllowed := expected[wizardObjectKey{catalog: "pg_class", oid: got.parentTable}]; parentAllowed {
				continue
			}
		}
		return fmt.Errorf("unexpected new object: %s/%d", key.catalog, key.oid)
	}
	return nil
}
