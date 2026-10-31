package documentverificationpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type Repository struct{ transactions transactionBeginner }

var _ dv.Repository = (*Repository)(nil)
var _ dv.OwnCaseReader = (*Repository)(nil)
var _ dv.SpaceDownloadRepository = (*Repository)(nil)
var _ identityverification.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository {
	if pool == nil {
		return &Repository{}
	}
	return &Repository{transactions: pool}
}

func newWithTransactionBeginner(transactions transactionBeginner) *Repository {
	return &Repository{transactions: transactions}
}

func (r *Repository) loadReviewerGrants(ctx context.Context, tx pgx.Tx, actor string) ([]dv.ReviewerGrant, error) {
	if r == nil || tx == nil || !validUUID(actor) {
		return nil, dv.ErrInvalid
	}
	rows, err := tx.Query(ctx, `SELECT reviewer_id::text,capability,scope_id::text
		FROM public.document_verification_reviewer_grant
		WHERE reviewer_id=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>clock_timestamp())
		ORDER BY capability,scope_id FOR SHARE`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]dv.ReviewerGrant, 0)
	for rows.Next() {
		var g dv.ReviewerGrant
		if err := rows.Scan(&g.ReviewerID, &g.Capability, &g.ScopeID); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func (r *Repository) CreateCase(ctx context.Context, actor string, purpose dv.Purpose, space string, build func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	if r == nil || r.transactions == nil || !validUUID(actor) || build == nil || (purpose != dv.PurposeIdentity && purpose != dv.PurposeSpace) || (purpose == dv.PurposeIdentity && space != "") || (purpose == dv.PurposeSpace && !validUUID(space)) {
		return dv.Snapshot{}, dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return dv.Snapshot{}, err
	}
	defer tx.Rollback(context.Background())
	auth := dv.Authorization{ActorID: actor, SubjectID: actor}
	if err = lockActor(ctx, tx, actor); err != nil {
		return dv.Snapshot{}, err
	}
	if purpose == dv.PurposeSpace {
		var owned bool
		owned, err = lockSpace(ctx, tx, space, actor)
		if err != nil {
			return dv.Snapshot{}, err
		}
		if !owned {
			return dv.Snapshot{}, dv.ErrCapability
		}
		auth.SpaceOwnerID = actor
	}
	x, err := build(auth)
	if err != nil {
		return dv.Snapshot{}, err
	}
	if x.Purpose != purpose || x.SubjectID != actor || x.SpaceID != space || !validUUID(x.ID) || x.State != dv.CaseDraft || len(x.Files) != 0 {
		return dv.Snapshot{}, dv.ErrInvalid
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,space_id,document_expires_at) VALUES($1,$2,$3,NULLIF($4,'')::uuid,$5)`, x.ID, purpose, actor, space, nullTime(x.DocumentExpiresAt)); err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_event(case_id,purpose,subject_id,actor_id,action,to_state,result_code,revision) VALUES($1,$2,$3,$4,'case_created',$5,$5,1)`, x.ID, purpose, actor, actor, x.State); err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	x.Revision = 1
	return x, nil
}

func (r *Repository) Transition(ctx context.Context, actor string, purpose dv.Purpose, id string, revision uint64, audit dv.TransitionAudit, change func(*dv.Snapshot, dv.Authorization) error) error {
	if r == nil || r.transactions == nil || !validUUID(actor) || !validUUID(id) || revision == 0 || change == nil {
		return dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = lockActor(ctx, tx, actor); err != nil {
		return err
	}
	var scopeSpace *string
	err = tx.QueryRow(ctx, `SELECT space_id::text FROM public.document_verification_case WHERE id=$1 AND purpose=$2`, id, purpose).Scan(&scopeSpace)
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.ErrNotFound
	}
	if err != nil {
		return err
	}
	var currentActorOwnsSpace bool
	if scopeSpace != nil {
		currentActorOwnsSpace, err = lockSpace(ctx, tx, *scopeSpace, actor)
		if err != nil {
			return err
		}
	}
	var x dv.Snapshot
	var spaceID *string
	err = tx.QueryRow(ctx, `SELECT id::text,purpose,subject_id::text,space_id::text,state,revision,COALESCE(reviewer_id::text,''),verified_at,verified_until,document_expires_at,revoked_at,superseded_at FROM public.document_verification_case WHERE id=$1 AND purpose=$2 FOR UPDATE`, id, purpose).Scan(&x.ID, &x.Purpose, &x.SubjectID, &spaceID, &x.State, &x.Revision, &x.ReviewerID, &nullableT{v: &x.VerifiedAt}, &nullableT{v: &x.VerifiedUntil}, &nullableT{v: &x.DocumentExpiresAt}, &nullablePT{v: &x.RevokedAt}, &nullablePT{v: &x.SupersededAt})
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.ErrNotFound
	}
	if err != nil {
		return err
	}
	if spaceID != nil {
		x.SpaceID = *spaceID
	}

	auth := dv.Authorization{ActorID: actor, SubjectID: x.SubjectID}
	if purpose == dv.PurposeSpace {
		if !currentActorOwnsSpace {
			return dv.ErrConflict
		}
		auth.SpaceOwnerID = actor
		if x.SupersededAt != nil || auth.SpaceOwnerID != x.SubjectID {
			return dv.ErrConflict
		}
	}
	if err = loadFiles(ctx, tx, &x); err != nil {
		return err
	}
	if purpose == dv.PurposeSpace {
		auth.IdentityCurrent, err = identityCurrent(ctx, tx, x.SubjectID)
		if err != nil {
			return err
		}
	}
	auth.ReviewerGrants, err = r.loadReviewerGrants(ctx, tx, actor)
	if err != nil {
		return err
	}
	if x.Revision != revision {
		return dv.ErrConflict
	}
	beforeFiles := append([]dv.File(nil), x.Files...)
	before := x.State
	if err = change(&x, auth); err != nil {
		return err
	}
	addedFiles, err := persistNewFiles(ctx, tx, id, purpose, beforeFiles, x.Files)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.document_verification_case SET state=$1,revision=revision+1,reviewer_id=NULLIF($2,'')::uuid,verified_at=$3,verified_until=$4,revoked_at=$5,updated_at=now() WHERE id=$6 AND revision=$7`, x.State, x.ReviewerID, nullTime(x.VerifiedAt), nullTime(x.VerifiedUntil), x.RevokedAt, id, revision); err != nil {
		return mapError(err)
	}
	if len(addedFiles) == 0 || before != x.State {
		if audit.Action == "" || audit.ResultCode == "" {
			return dv.ErrInvalid
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_event(case_id,purpose,subject_id,actor_id,action,from_state,to_state,result_code,reason_code,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'') ,$10)`, id, purpose, x.SubjectID, actor, audit.Action, before, x.State, audit.ResultCode, audit.ReasonCode, revision+1); err != nil {
			return mapError(err)
		}
	}
	for _, fileID := range addedFiles {
		if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_event(case_id,purpose,subject_id,file_id,actor_id,action,from_state,to_state,result_code,revision) VALUES($1,$2,$3,$4,$5,'file_added',$6,$6,'pendiente',$7)`, id, purpose, x.SubjectID, fileID, actor, x.State, revision+1); err != nil {
			return mapError(err)
		}
	}
	return mapError(tx.Commit(ctx))
}

func (r *Repository) ListOwnCases(ctx context.Context, actor string) ([]dv.Snapshot, error) {
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
	rows, err := tx.Query(ctx, `SELECT c.id::text,c.purpose,c.subject_id::text,c.space_id::text,c.state,c.revision,COALESCE(c.reviewer_id::text,''),c.verified_at,c.verified_until,c.document_expires_at,c.revoked_at,c.superseded_at FROM public.document_verification_case c WHERE c.subject_id=$1 AND c.superseded_at IS NULL AND (c.purpose='identity' OR (c.purpose='space' AND public.document_verification_space_owned_by(c.space_id,$1::uuid))) ORDER BY c.updated_at DESC,c.id`, actor)
	if err != nil {
		return nil, mapError(err)
	}
	cases := make([]dv.Snapshot, 0)
	for rows.Next() {
		x, scanErr := scanSnapshot(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		cases = append(cases, x)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}
	for i := range cases {
		if err = loadFiles(ctx, tx, &cases[i]); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return cases, nil
}

func (r *Repository) LoadOwnCase(ctx context.Context, actor, id string) (dv.Snapshot, error) {
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
	x, err := scanSnapshot(tx.QueryRow(ctx, `SELECT c.id::text,c.purpose,c.subject_id::text,c.space_id::text,c.state,c.revision,COALESCE(c.reviewer_id::text,''),c.verified_at,c.verified_until,c.document_expires_at,c.revoked_at,c.superseded_at FROM public.document_verification_case c WHERE c.id=$1 AND c.subject_id=$2 AND c.superseded_at IS NULL AND (c.purpose='identity' OR (c.purpose='space' AND public.document_verification_space_owned_by(c.space_id,$2::uuid)))`, id, actor))
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.Snapshot{}, dv.ErrNotFound
	}
	if err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	if err = loadFiles(ctx, tx, &x); err != nil {
		return dv.Snapshot{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return dv.Snapshot{}, mapError(err)
	}
	return x, nil
}

func (r *Repository) LoadSpaceCaseForDownload(ctx context.Context, actor, id string) (dv.Snapshot, dv.Authorization, error) {
	if r == nil || r.transactions == nil || !validUUID(actor) || !validUUID(id) {
		return dv.Snapshot{}, dv.Authorization{}, dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return dv.Snapshot{}, dv.Authorization{}, err
	}
	defer tx.Rollback(context.Background())
	var x dv.Snapshot
	err = tx.QueryRow(ctx, `SELECT c.id::text,c.purpose,c.subject_id::text,c.space_id::text,c.state,c.revision,COALESCE(c.reviewer_id::text,''),c.verified_at,c.verified_until,c.document_expires_at,c.revoked_at,c.superseded_at FROM public.document_verification_case c WHERE c.id=$1 AND c.purpose='space' AND c.superseded_at IS NULL AND public.document_verification_lock_space_owner(c.space_id,$2::uuid)`, id, actor).Scan(&x.ID, &x.Purpose, &x.SubjectID, &x.SpaceID, &x.State, &x.Revision, &x.ReviewerID, &nullableT{v: &x.VerifiedAt}, &nullableT{v: &x.VerifiedUntil}, &nullableT{v: &x.DocumentExpiresAt}, &nullablePT{v: &x.RevokedAt}, &nullablePT{v: &x.SupersededAt})
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.Snapshot{}, dv.Authorization{}, dv.ErrNotFound
	}
	if err != nil {
		return dv.Snapshot{}, dv.Authorization{}, mapError(err)
	}
	a := dv.Authorization{ActorID: actor, SubjectID: x.SubjectID, SpaceOwnerID: actor}
	if err = loadFiles(ctx, tx, &x); err != nil {
		return dv.Snapshot{}, a, err
	}
	if err = tx.Commit(ctx); err != nil {
		return dv.Snapshot{}, a, mapError(err)
	}
	return x, a, nil
}

type nullableT struct{ v *time.Time }

func (n *nullableT) Scan(src any) error {
	if src == nil {
		*n.v = time.Time{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*n.v = v
		return nil
	default:
		return fmt.Errorf("unexpected timestamp %T", src)
	}
}

type nullablePT struct{ v **time.Time }

func (n *nullablePT) Scan(src any) error {
	if src == nil {
		*n.v = nil
		return nil
	}
	v, ok := src.(time.Time)
	if !ok {
		return fmt.Errorf("unexpected timestamp %T", src)
	}
	*n.v = &v
	return nil
}

type snapshotScanner interface {
	Scan(...any) error
}

func scanSnapshot(row snapshotScanner) (dv.Snapshot, error) {
	var x dv.Snapshot
	var spaceID *string
	err := row.Scan(&x.ID, &x.Purpose, &x.SubjectID, &spaceID, &x.State, &x.Revision, &x.ReviewerID, &nullableT{v: &x.VerifiedAt}, &nullableT{v: &x.VerifiedUntil}, &nullableT{v: &x.DocumentExpiresAt}, &nullablePT{v: &x.RevokedAt}, &nullablePT{v: &x.SupersededAt})
	if spaceID != nil {
		x.SpaceID = *spaceID
	}
	return x, err
}

func validUUID(s string) bool {
	u, e := uuid.Parse(s)
	return e == nil && u != uuid.Nil && u.String() == s
}
func lockActor(ctx context.Context, tx pgx.Tx, id string) error {
	var active bool
	e := tx.QueryRow(ctx, `SELECT public.document_verification_lock_active_actor($1::uuid)`, id).Scan(&active)
	if e == nil && !active {
		return dv.ErrCapability
	}
	return mapError(e)
}
func lockSpace(ctx context.Context, tx pgx.Tx, id, expectedOwner string) (bool, error) {
	var owned bool
	e := tx.QueryRow(ctx, `SELECT public.document_verification_lock_space_owner($1::uuid,$2::uuid)`, id, expectedOwner).Scan(&owned)
	return owned, mapError(e)
}
func loadFiles(ctx context.Context, tx pgx.Tx, x *dv.Snapshot) error {
	rows, e := tx.Query(ctx, `SELECT id::text,state,size_bytes,mime_type,pages,content_sha256,retention_at FROM public.document_verification_file WHERE case_id=$1 ORDER BY id`, x.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	x.Files = []dv.File{}
	for rows.Next() {
		var f dv.File
		var contentSHA256 []byte
		if e = rows.Scan(&f.ID, &f.State, &f.Size, &f.MIME, &f.Pages, &contentSHA256, &nullableT{v: &f.RetentionAt}); e != nil {
			return e
		}
		if len(contentSHA256) != len(f.ContentSHA256) {
			return errors.New("persisted document content hash has invalid length")
		}
		copy(f.ContentSHA256[:], contentSHA256)
		x.Files = append(x.Files, f)
	}
	return rows.Err()
}

func persistNewFiles(ctx context.Context, tx pgx.Tx, caseID string, purpose dv.Purpose, previous, current []dv.File) ([]string, error) {
	if len(current) < len(previous) || dv.ValidateQuota(purpose, current) != nil {
		return nil, dv.ErrInvalid
	}
	prior := make(map[string]dv.File, len(previous))
	for _, file := range previous {
		prior[file.ID] = file
	}
	added := make([]string, 0, len(current)-len(previous))
	for _, file := range current {
		if old, exists := prior[file.ID]; exists {
			if old != file {
				return nil, dv.ErrInvalid
			}
			delete(prior, file.ID)
			continue
		}
		if file.State != dv.FilePending {
			return nil, dv.ErrInvalid
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.document_verification_file(id,case_id,size_bytes,mime_type,pages,content_sha256) VALUES($1,$2,$3,$4,$5,$6)`, file.ID, caseID, file.Size, file.MIME, file.Pages, file.ContentSHA256[:]); err != nil {
			return nil, mapError(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind) VALUES($1,'scan')`, file.ID); err != nil {
			return nil, mapError(err)
		}
		added = append(added, file.ID)
	}
	if len(prior) != 0 {
		return nil, dv.ErrInvalid
	}
	return added, nil
}

func identityCurrent(ctx context.Context, tx pgx.Tx, subject string) (bool, error) {
	var id string
	e := tx.QueryRow(ctx, `SELECT id::text FROM public.document_verification_case WHERE purpose='identity' AND subject_id=$1 AND state='aprobada' AND verified_until>now() ORDER BY verified_until DESC,id LIMIT 1 FOR SHARE`, subject).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	return e == nil, e
}

func (r *Repository) CurrentForAccount(ctx context.Context, accountID string) (identityverification.Verification, error) {
	if r == nil || r.transactions == nil || !validUUID(accountID) {
		return identityverification.Verification{}, identityverification.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return identityverification.Verification{}, err
	}
	defer tx.Rollback(context.Background())
	var current identityverification.Verification
	current.Kind = identityverification.KindIdentityDocument
	current.ReviewMethod = identityverification.ReviewMethodManualDocuments
	err = tx.QueryRow(ctx, `SELECT subject_id::text,state,verified_at,verified_until,revoked_at
		FROM public.document_verification_case
		WHERE purpose='identity' AND subject_id=$1 AND state IN ('aprobada','revocada','vencida')
		ORDER BY updated_at DESC,id DESC LIMIT 1`, accountID).Scan(
		&current.SubjectID, &current.State,
		&nullableT{v: &current.VerifiedAt}, &nullableT{v: &current.ExpiresAt}, &nullablePT{v: &current.RevokedAt},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return identityverification.Verification{}, identityverification.ErrNotFound
	}
	if err != nil {
		return identityverification.Verification{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return identityverification.Verification{}, err
	}
	return current, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
func mapError(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) && (p.Code == "23505" || p.Code == "23514" || p.Code == "40001") {
		return dv.ErrConflict
	}
	return e
}
