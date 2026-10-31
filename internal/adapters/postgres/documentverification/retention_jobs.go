package documentverificationpg

import (
	"context"
	"errors"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
	"time"
)

type RetentionClaim struct {
	JobID, FileID, CaseID, SubjectID, SpaceID string
	Purpose                                   dv.Purpose
	Attempt                                   int16
	LeaseUntil                                time.Time
}

func (r *Repository) ClaimReadyRetentionJob(ctx context.Context, lease time.Duration) (*RetentionClaim, error) {
	return r.claimReadyRetentionJob(ctx, lease, "", time.Time{})
}

func (r *Repository) claimReadyRetentionJob(ctx context.Context, lease time.Duration, onlyJobID string, readyBefore time.Time) (*RetentionClaim, error) {
	if r == nil || r.transactions == nil || lease <= 0 {
		return nil, dv.ErrInvalid
	}
	if onlyJobID != "" && !validUUID(onlyJobID) {
		return nil, dv.ErrInvalid
	}
	var jobFilter, readyBeforeFilter any
	if onlyJobID != "" {
		jobFilter = onlyJobID
	}
	if !readyBefore.IsZero() {
		readyBeforeFilter = readyBefore.UTC()
	}
	tx, e := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	var c RetentionClaim
	e = tx.QueryRow(ctx, `SELECT j.id::text,f.id::text,c.id::text,c.subject_id::text,c.purpose,COALESCE(c.space_id::text,''),j.attempts
FROM public.document_verification_job j
JOIN public.document_verification_file f ON f.id=j.file_id
JOIN public.document_verification_case c ON c.id=f.case_id
WHERE j.kind='retention_delete'
  AND ($1::uuid IS NULL OR j.id=$1::uuid)
  AND f.retention_at<=clock_timestamp()
  AND NOT EXISTS (
      SELECT 1 FROM public.document_verification_job scan
      WHERE scan.file_id=f.id AND scan.kind='scan'
        AND scan.state IN ('pendiente','procesando','reintentar')
  )
  AND ((j.state IN ('pendiente','reintentar') AND j.available_at<=COALESCE($2::timestamptz,clock_timestamp()))
       OR (j.state='procesando' AND j.lease_until<=clock_timestamp()))
ORDER BY j.available_at,j.id LIMIT 1`, jobFilter, readyBeforeFilter).Scan(&c.JobID, &c.FileID, &c.CaseID, &c.SubjectID, &c.Purpose, &c.SpaceID, &c.Attempt)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if c.SpaceID != "" {
		var ok bool
		if e = tx.QueryRow(ctx, `SELECT public.document_verification_lock_space_row($1::uuid)`, c.SpaceID).Scan(&ok); e != nil {
			return nil, e
		}
		if !ok {
			return nil, tx.Commit(ctx)
		}
	}
	if e = tx.QueryRow(ctx, `SELECT id FROM public.document_verification_case WHERE id=$1 FOR UPDATE`, c.CaseID).Scan(new(string)); e != nil {
		return nil, e
	}
	if e = tx.QueryRow(ctx, `SELECT id FROM public.document_verification_file WHERE id=$1 AND retention_at<=clock_timestamp() FOR UPDATE`, c.FileID).Scan(new(string)); e != nil {
		return nil, e
	}
	var activeScan bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan' AND state IN ('pendiente','procesando','reintentar'))`, c.FileID).Scan(&activeScan); e != nil {
		return nil, e
	}
	if activeScan {
		return nil, tx.Commit(ctx)
	}
	e = tx.QueryRow(ctx, `UPDATE public.document_verification_job SET state='procesando',attempts=LEAST(attempts+1,5),lease_until=clock_timestamp()+$2::interval,updated_at=clock_timestamp() WHERE id=$1 AND kind='retention_delete' AND ((state IN ('pendiente','reintentar') AND available_at<=COALESCE($3::timestamptz,clock_timestamp())) OR (state='procesando' AND lease_until<=clock_timestamp())) RETURNING attempts,lease_until`, c.JobID, lease.String(), readyBeforeFilter).Scan(&c.Attempt, &c.LeaseUntil)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return &c, nil
}
func (r *Repository) finishRetention(ctx context.Context, c RetentionClaim, deleteObject func(context.Context) error, retry time.Duration) error {
	if r == nil || r.transactions == nil || !validUUID(c.JobID) || !validUUID(c.CaseID) || !validUUID(c.FileID) || c.LeaseUntil.IsZero() {
		return dv.ErrInvalid
	}
	tx, e := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	var purpose dv.Purpose
	var subject, space string
	if e = tx.QueryRow(ctx, `SELECT purpose,subject_id::text,COALESCE(space_id::text,'') FROM public.document_verification_case WHERE id=$1`, c.CaseID).Scan(&purpose, &subject, &space); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return dv.ErrConflict
		}
		return e
	}
	if space != "" {
		var ok bool
		if e = tx.QueryRow(ctx, `SELECT public.document_verification_lock_space_row($1::uuid)`, space).Scan(&ok); e != nil {
			return e
		}
		if !ok {
			return dv.ErrConflict
		}
	}
	var caseState string
	var revision int64
	if e = tx.QueryRow(ctx, `SELECT purpose,subject_id::text,COALESCE(space_id::text,''),state,revision FROM public.document_verification_case WHERE id=$1 FOR UPDATE`, c.CaseID).Scan(&purpose, &subject, &space, &caseState, &revision); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return dv.ErrConflict
		}
		return e
	}
	if purpose != c.Purpose || subject != c.SubjectID || space != c.SpaceID {
		return dv.ErrConflict
	}
	var fileID string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM public.document_verification_file WHERE id=$1 AND case_id=$2 AND retention_at<=clock_timestamp() FOR UPDATE`, c.FileID, c.CaseID).Scan(&fileID); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return dv.ErrConflict
		}
		return e
	}
	var state string
	var leaseMatches, leaseLive bool
	if e = tx.QueryRow(ctx, `SELECT state,COALESCE(lease_until=$2,false),COALESCE(lease_until>clock_timestamp(),false) FROM public.document_verification_job WHERE id=$1 AND file_id=$3 AND kind='retention_delete' FOR UPDATE`, c.JobID, c.LeaseUntil, c.FileID).Scan(&state, &leaseMatches, &leaseLive); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return dv.ErrConflict
		}
		return e
	}
	if state != "procesando" || !leaseMatches || !leaseLive {
		return dv.ErrConflict
	}
	var activeScan bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.document_verification_job WHERE file_id=$1 AND kind='scan' AND state IN ('pendiente','procesando','reintentar'))`, c.FileID).Scan(&activeScan); e != nil {
		return e
	}
	if activeScan {
		return dv.ErrConflict
	}
	if e = deleteObject(ctx); e != nil {
		tag, updateErr := tx.Exec(ctx, `UPDATE public.document_verification_job SET state='reintentar',available_at=clock_timestamp()+$3::interval,lease_until=NULL,last_error_code=$4,updated_at=clock_timestamp() WHERE id=$1 AND file_id=$2 AND kind='retention_delete' AND state='procesando' AND lease_until=$5`, c.JobID, c.FileID, retry.String(), retentionStoreError, c.LeaseUntil)
		e = updateErr
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return dv.ErrConflict
		}
		return tx.Commit(ctx)
	}
	// The job row remains FOR UPDATE-locked, so no later claimant can steal
	// this exact lease while storage deletion is in progress.
	tag, e := tx.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,last_error_code=NULL,updated_at=clock_timestamp() WHERE id=$1 AND file_id=$2 AND kind='retention_delete' AND state='procesando' AND lease_until=$3`, c.JobID, c.FileID, c.LeaseUntil)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return dv.ErrConflict
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.document_verification_event(case_id,purpose,subject_id,file_id,actor_kind,action,from_state,to_state,result_code,reason_code,revision) VALUES($1,$2,$3,$4,'system','retention_deleted',$5,$5,'eliminado','retencion_vencida',$6)`, c.CaseID, purpose, subject, c.FileID, caseState, revision); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// ProcessClaimedRetentionJob fences storage deletion with the persisted claim
// while the space, case, file and job rows remain locked in that order.
func (r *Repository) ProcessClaimedRetentionJob(ctx context.Context, c RetentionClaim, deleteObject func(context.Context) error, retry time.Duration) error {
	if deleteObject == nil || retry <= 0 {
		return dv.ErrInvalid
	}
	return r.finishRetention(ctx, c, deleteObject, retry)
}
