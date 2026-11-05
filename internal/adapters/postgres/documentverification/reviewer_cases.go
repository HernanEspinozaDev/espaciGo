package documentverificationpg

import (
	"context"
	"errors"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

var _ dv.ReviewerCaseReader = (*Repository)(nil)

const reviewerSnapshotColumns = `c.id::text,c.purpose,c.subject_id::text,c.space_id::text,c.state,c.revision,COALESCE(c.reviewer_id::text,''),c.verified_at,c.verified_until,c.document_expires_at,c.revoked_at,c.superseded_at`

// LoadReviewerCase returns metadata only for an exact, active reviewer grant.
func (r *Repository) LoadReviewerCase(ctx context.Context, actor, id string) (dv.Snapshot, error) {
	if r == nil || r.transactions == nil || !validUUID(actor) || !validUUID(id) {
		return dv.Snapshot{}, dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return dv.Snapshot{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockActor(ctx, tx, actor); err != nil {
		return dv.Snapshot{}, err
	}
	var scope *string
	var subject string
	if err = tx.QueryRow(ctx, `SELECT space_id::text,subject_id::text FROM public.document_verification_case WHERE id=$1`, id).Scan(&scope, &subject); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dv.Snapshot{}, dv.ErrNotFound
		}
		return dv.Snapshot{}, err
	}
	if scope != nil {
		owned, e := lockSpace(ctx, tx, *scope, subject)
		if e != nil {
			return dv.Snapshot{}, e
		}
		if !owned {
			return dv.Snapshot{}, dv.ErrNotFound
		}
	}
	x, err := scanSnapshot(tx.QueryRow(ctx, `SELECT `+reviewerSnapshotColumns+` FROM public.document_verification_case c WHERE c.id=$1 FOR SHARE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	if err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	if x.SubjectID != subject || x.SubjectID == actor {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	grants, err := r.loadReviewerGrants(ctx, tx, actor)
	if err != nil {
		return dv.Snapshot{}, err
	}
	if !reviewerGrantMatches(grants, actor, x) {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	if err = loadFiles(ctx, tx, &x); err != nil {
		return dv.Snapshot{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	return x, nil
}

func reviewerGrantMatches(gs []dv.ReviewerGrant, actor string, x dv.Snapshot) bool {
	if !validUUID(actor) || !validUUID(x.SubjectID) || x.SubjectID == actor || x.SupersededAt != nil {
		return false
	}
	capability, scope := dv.CapabilityIdentity, x.SubjectID
	if x.Purpose == dv.PurposeSpace {
		capability, scope = dv.CapabilitySpace, x.SpaceID
	} else if x.Purpose != dv.PurposeIdentity {
		return false
	}
	if !validUUID(scope) {
		return false
	}
	capabilities := make(map[dv.Capability]bool, 2)
	matched := false
	for _, g := range gs {
		if g.ReviewerID != actor || !validUUID(g.ScopeID) || (g.Capability != dv.CapabilityIdentity && g.Capability != dv.CapabilitySpace) {
			return false
		}
		capabilities[g.Capability] = true
		if g.Capability == capability && g.ScopeID == scope {
			matched = true
		}
	}
	return len(capabilities) == 1 && matched
}

func (r *Repository) ListReviewerCases(ctx context.Context, actor string) ([]dv.Snapshot, error) {
	if r == nil || r.transactions == nil || !validUUID(actor) {
		return nil, dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = lockActor(ctx, tx, actor); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT `+reviewerSnapshotColumns+` FROM public.document_verification_case c
		WHERE c.subject_id<>$1 AND c.superseded_at IS NULL AND EXISTS (
		 SELECT 1 FROM public.document_verification_reviewer_grant g
		 WHERE g.reviewer_id=$1 AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>clock_timestamp())
		 AND ((c.purpose='identity' AND g.capability='review_identity' AND g.scope_kind='identity_subject' AND g.scope_id=c.subject_id)
		   OR (c.purpose='space' AND g.capability='review_space' AND g.scope_kind='space' AND g.scope_id=c.space_id))
		 AND NOT EXISTS (
		  SELECT 1 FROM public.document_verification_reviewer_grant other
		  WHERE other.reviewer_id=$1 AND other.capability<>g.capability AND other.revoked_at IS NULL
		    AND (other.expires_at IS NULL OR other.expires_at>clock_timestamp())
		 )
		) ORDER BY c.space_id NULLS FIRST,c.id`, actor)
	if err != nil {
		return nil, mapError(err)
	}
	cases := make([]dv.Snapshot, 0)
	for rows.Next() {
		x, e := scanSnapshot(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		cases = append(cases, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := make([]dv.Snapshot, 0)
	for _, candidate := range cases {
		if candidate.Purpose == dv.PurposeSpace {
			owned, e := lockSpace(ctx, tx, candidate.SpaceID, candidate.SubjectID)
			if e != nil {
				return nil, e
			}
			if !owned {
				continue
			}
		}
		x, e := scanSnapshot(tx.QueryRow(ctx, `SELECT `+reviewerSnapshotColumns+` FROM public.document_verification_case c WHERE c.id=$1 AND c.subject_id<>$2 AND c.superseded_at IS NULL FOR SHARE`, candidate.ID, actor))
		if errors.Is(e, pgx.ErrNoRows) {
			continue
		}
		if e != nil {
			return nil, mapError(e)
		}
		grants, e := r.loadReviewerGrants(ctx, tx, actor)
		if e != nil {
			return nil, e
		}
		if !reviewerGrantMatches(grants, actor, x) {
			continue
		}
		if e = loadFiles(ctx, tx, &x); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (r *Repository) WithReviewerCaseForRead(ctx context.Context, actor, id string, callback func(context.Context, dv.Snapshot, dv.Authorization) error) error {
	if callback == nil || r == nil || r.transactions == nil || !validUUID(actor) || !validUUID(id) {
		return dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = lockActor(ctx, tx, actor); err != nil {
		return err
	}
	var subject string
	var space *string
	err = tx.QueryRow(ctx, `SELECT subject_id::text,space_id::text FROM public.document_verification_case WHERE id=$1`, id).Scan(&subject, &space)
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.ErrNotFound
	}
	if err != nil {
		return err
	}
	var owner string
	if space != nil {
		owner = subject
		owned, e := lockSpace(ctx, tx, *space, owner)
		if e != nil {
			return e
		}
		if !owned {
			return dv.ErrNotFound
		}
	}
	x, err := scanSnapshot(tx.QueryRow(ctx, `SELECT `+reviewerSnapshotColumns+` FROM public.document_verification_case c WHERE c.id=$1 FOR SHARE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.ErrNotFound
	}
	if err != nil {
		return mapError(err)
	}
	if x.SubjectID != subject || x.SubjectID == actor {
		return dv.ErrNotFound
	}
	grants, err := r.loadReviewerGrants(ctx, tx, actor)
	if err != nil {
		return err
	}
	if !reviewerGrantMatches(grants, actor, x) {
		return dv.ErrNotFound
	}
	if err = loadFiles(ctx, tx, &x); err != nil {
		return err
	}
	auth := dv.Authorization{ActorID: actor, SubjectID: x.SubjectID, SpaceOwnerID: owner, ReviewerGrants: grants}
	if err = callback(ctx, x, auth); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}
