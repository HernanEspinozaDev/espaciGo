package documentverificationpg

import (
	"context"
	"errors"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type ScanClaim struct {
	JobID, FileID, CaseID, SubjectID string
	Purpose                          dv.Purpose
	SpaceID                          string
	Size                             int64
	MIME                             string
	Pages                            int
	ContentSHA256                    [32]byte
	Attempt                          int16
	CaseRevision                     int64
	LeaseUntil                       time.Time
	PriorRetentionAt                 *time.Time
}

type ScanOutcome struct {
	State          dv.FileState
	ErrorCode      string
	ReasonCode     dv.ReasonCode
	ScannerVersion string
	SignaturesAt   time.Time
}

func (r *Repository) VerifyScanClaim(ctx context.Context, claim ScanClaim) (bool, error) {
	if ctx == nil || r == nil || r.transactions == nil || !validUUID(claim.JobID) || !validUUID(claim.FileID) || !validUUID(claim.CaseID) || !validUUID(claim.SubjectID) || claim.CaseRevision <= 0 || claim.Attempt <= 0 || claim.LeaseUntil.IsZero() {
		return false, dv.ErrInvalid
	}
	switch claim.Purpose {
	case dv.PurposeIdentity:
		if claim.SpaceID != "" {
			return false, dv.ErrInvalid
		}
	case dv.PurposeSpace:
		if !validUUID(claim.SpaceID) {
			return false, dv.ErrInvalid
		}
	default:
		return false, dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	var current bool
	var space any
	if claim.SpaceID != "" {
		space = claim.SpaceID
	}
	err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1
		FROM public.document_verification_job j
		JOIN public.document_verification_file f ON f.id=j.file_id
		JOIN public.document_verification_case c ON c.id=f.case_id
		WHERE j.id=$1 AND j.file_id=$2 AND j.kind='scan'
		  AND j.state='procesando' AND j.attempts=$3
		  AND j.lease_until=$4 AND j.lease_until>clock_timestamp()
		  AND f.id=$2 AND f.state='pendiente' AND f.retention_at IS NOT DISTINCT FROM $10::timestamptz
		  AND c.id=$5 AND c.purpose=$6 AND c.subject_id=$7
		  AND c.revision=$8 AND c.superseded_at IS NULL
		  AND (($6='identity' AND c.space_id IS NULL)
		       OR ($6='space' AND c.space_id=$9::uuid))
	)`, claim.JobID, claim.FileID, claim.Attempt, claim.LeaseUntil, claim.CaseID, claim.Purpose, claim.SubjectID, claim.CaseRevision, space, claim.PriorRetentionAt).Scan(&current)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return current, nil
}

func (r *Repository) ClaimReadyScanJob(ctx context.Context, lease time.Duration) (*ScanClaim, error) {
	if r == nil || r.transactions == nil || lease <= 0 {
		return nil, dv.ErrInvalid
	}
	lease = normalizeScanLease(lease)
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	var c ScanClaim
	var hash []byte
	// Select without a locking ownership predicate: it can lock multiple spaces
	// while filtering. Lock only the chosen space below, then revalidate its case.
	err = tx.QueryRow(ctx, `SELECT j.id::text,f.id::text,c.id::text,c.subject_id::text,c.purpose,COALESCE(c.space_id::text,''),f.size_bytes,f.mime_type,f.pages,f.content_sha256,c.revision FROM public.document_verification_job j JOIN public.document_verification_file f ON f.id=j.file_id JOIN public.document_verification_case c ON c.id=f.case_id WHERE j.kind='scan' AND ((j.state IN ('pendiente','reintentar') AND j.available_at<=clock_timestamp() AND f.state IN ('pendiente','bloqueado')) OR (j.state='procesando' AND j.lease_until<=clock_timestamp())) AND c.superseded_at IS NULL AND ((c.purpose='identity' AND c.space_id IS NULL) OR (c.purpose='space' AND c.space_id IS NOT NULL AND EXISTS (SELECT 1 FROM public.espacio s WHERE s.id=c.space_id AND s.propietario_id=c.subject_id))) ORDER BY j.available_at,j.id LIMIT 1`).Scan(&c.JobID, &c.FileID, &c.CaseID, &c.SubjectID, &c.Purpose, &c.SpaceID, &c.Size, &c.MIME, &c.Pages, &hash, &c.CaseRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(hash) != 32 {
		return nil, errors.New("invalid document hash")
	}
	copy(c.ContentSHA256[:], hash)
	if c.SpaceID != "" {
		owned, lockErr := lockSpace(ctx, tx, c.SpaceID, c.SubjectID)
		if lockErr != nil {
			return nil, lockErr
		}
		if !owned {
			return nil, tx.Commit(ctx)
		}
	}
	var revision int64
	var superseded *time.Time
	err = tx.QueryRow(ctx, `SELECT revision,superseded_at FROM public.document_verification_case WHERE id=$1 FOR UPDATE`, c.CaseID).Scan(&revision, &nullablePT{v: &superseded})
	if err != nil {
		return nil, err
	}
	if revision != c.CaseRevision || superseded != nil {
		return nil, tx.Commit(ctx)
	}
	if err = tx.QueryRow(ctx, `SELECT retention_at FROM public.document_verification_file WHERE id=$1 FOR UPDATE`, c.FileID).Scan(&nullablePT{v: &c.PriorRetentionAt}); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT attempts FROM public.document_verification_job WHERE id=$1 FOR UPDATE`, c.JobID).Scan(&c.Attempt); err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, `UPDATE public.document_verification_job SET state='procesando',attempts=LEAST(attempts+1,5),lease_until=clock_timestamp()+$2::interval,updated_at=clock_timestamp() WHERE id=$1 AND ((state IN ('pendiente','reintentar') AND available_at<=clock_timestamp()) OR (state='procesando' AND lease_until<=clock_timestamp())) RETURNING attempts,lease_until`, c.JobID, lease.String()).Scan(&c.Attempt, &c.LeaseUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Keep the prior retention deadline/reason visible while this scan is active:
	// an ownership transfer may fence the claim and must preserve that policy.
	if _, err = tx.Exec(ctx, `UPDATE public.document_verification_file SET state='pendiente',last_error_code=NULL,updated_at=clock_timestamp() WHERE id=$1 AND state='bloqueado'`, c.FileID); err != nil {
		return nil, err
	}
	if err = cancelScanRetention(ctx, tx, c.FileID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) RecordScanRetry(ctx context.Context, claim ScanClaim, errorCode string, retryAfter time.Duration) error {
	if !validUUID(claim.JobID) || !validUUID(claim.FileID) || retryAfter < 0 || !scanErrorCode(errorCode) {
		return dv.ErrInvalid
	}
	return r.finishScan(ctx, claim, func(ctx context.Context, tx pgx.Tx, now time.Time) error {
		var previousReason, previousError string
		var previousRetentionAt *time.Time
		var inspectedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT retention_at,COALESCE(retention_reason_code,''),COALESCE(last_error_code,''),inspected_at FROM public.document_verification_file WHERE id=$1`, claim.FileID).Scan(&nullablePT{v: &previousRetentionAt}, &previousReason, &previousError, &nullablePT{v: &inspectedAt}); err != nil {
			return err
		}
		retentionDeadline, retentionReason := scanRetryRetentionPolicy(now, previousRetentionAt, previousReason, previousError, inspectedAt)
		retentionDeadline = earlierRetentionDeadline(retentionDeadline, claim.PriorRetentionAt)
		if claim.Attempt >= 5 {
			_, err := tx.Exec(ctx, `UPDATE public.document_verification_file SET state='error',last_error_code=$2,retention_at=$3,retention_reason_code=$4,updated_at=$5 WHERE id=$1`, claim.FileID, errorCode, retentionDeadline, retentionReason, now)
			if err != nil {
				return err
			}
			if err := scheduleScanRetention(ctx, tx, claim.FileID, retentionDeadline); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,last_error_code=$2,updated_at=$3 WHERE id=$1`, claim.JobID, errorCode, now)
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.document_verification_file SET state='bloqueado',last_error_code=$2,retention_at=$3,retention_reason_code=$4,updated_at=$5 WHERE id=$1`, claim.FileID, errorCode, retentionDeadline, retentionReason, now)
		if err != nil {
			return err
		}
		if err := scheduleScanRetention(ctx, tx, claim.FileID, retentionDeadline); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.document_verification_job SET state='reintentar',available_at=$3::timestamptz + $4::interval,lease_until=NULL,last_error_code=$2,updated_at=$3::timestamptz WHERE id=$1`, claim.JobID, errorCode, now, retryAfter.String())
		return err
	})
}

func (r *Repository) RecordScanOutcome(ctx context.Context, claim ScanClaim, outcome ScanOutcome) error {
	clean := outcome.State == dv.FileClean
	valid := clean && outcome.ScannerVersion != "" && !outcome.SignaturesAt.IsZero() && outcome.ErrorCode == "" || outcome.State == dv.FileBlocked && outcome.ScannerVersion != "" && !outcome.SignaturesAt.IsZero() && outcome.ErrorCode == "malware_detected" && outcome.ReasonCode == dv.ReasonMalwareDetected || outcome.State == dv.FileError && scanErrorCode(outcome.ErrorCode) && outcome.ErrorCode != "malware_detected"
	if !valid || !validUUID(claim.JobID) || !validUUID(claim.CaseID) || !validUUID(claim.FileID) {
		return dv.ErrInvalid
	}
	return r.finishScan(ctx, claim, func(ctx context.Context, tx pgx.Tx, now time.Time) error {
		var state dv.FileState
		retention := now.Add(7 * 24 * time.Hour)
		reason := "inspection_error"
		code := outcome.ErrorCode
		if clean {
			state = dv.FileClean
			retention = time.Time{}
			reason = ""
			code = ""
		} else if outcome.State == dv.FileBlocked {
			state = dv.FileBlocked
			code = "malware_detected"
			var previous *time.Time
			var previousReason string
			if err := tx.QueryRow(ctx, `SELECT retention_at,COALESCE(retention_reason_code,'') FROM public.document_verification_file WHERE id=$1`, claim.FileID).Scan(&nullablePT{v: &previous}, &previousReason); err != nil {
				return err
			}
			retention, reason = scanRetryRetentionPolicy(now, previous, previousReason, "malware_detected", &now)
			retention = earlierRetentionDeadline(retention, claim.PriorRetentionAt)
		} else if outcome.State == dv.FileError {
			state = dv.FileError
			var previousReason, previousError string
			var previousRetentionAt *time.Time
			var inspectedAt *time.Time
			if err := tx.QueryRow(ctx, `SELECT retention_at,COALESCE(retention_reason_code,''),COALESCE(last_error_code,''),inspected_at FROM public.document_verification_file WHERE id=$1`, claim.FileID).Scan(&nullablePT{v: &previousRetentionAt}, &previousReason, &previousError, &nullablePT{v: &inspectedAt}); err != nil {
				return err
			}
			retention, reason = scanRetryRetentionPolicy(now, previousRetentionAt, previousReason, previousError, inspectedAt)
			retention = earlierRetentionDeadline(retention, claim.PriorRetentionAt)
		}
		_, err := tx.Exec(ctx, `UPDATE public.document_verification_file SET state=$2,last_error_code=NULLIF($3,''),scanner_version=NULLIF($4,''),signatures_at=NULLIF($5,'epoch'::timestamptz),inspected_at=$6,retention_at=NULLIF($7,'epoch'::timestamptz),retention_reason_code=NULLIF($8,''),updated_at=$6 WHERE id=$1`, claim.FileID, state, code, outcome.ScannerVersion, func() time.Time {
			if outcome.SignaturesAt.IsZero() {
				return time.Unix(0, 0)
			}
			return outcome.SignaturesAt
		}(), now, func() time.Time {
			if retention.IsZero() {
				return time.Unix(0, 0)
			}
			return retention
		}(), reason)
		if err != nil {
			return err
		}
		if retention.IsZero() {
			if err := cancelScanRetention(ctx, tx, claim.FileID); err != nil {
				return err
			}
		} else if err := scheduleScanRetention(ctx, tx, claim.FileID, retention); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.document_verification_case SET revision=revision+1,updated_at=$2 WHERE id=$1`, claim.CaseID, now); err != nil {
			return err
		}
		var purpose dv.Purpose
		var subject, caseState string
		var rev int64
		if err = tx.QueryRow(ctx, `SELECT purpose,subject_id::text,state,revision FROM public.document_verification_case WHERE id=$1`, claim.CaseID).Scan(&purpose, &subject, &caseState, &rev); err != nil {
			return err
		}
		why := outcome.ReasonCode
		if clean {
			why = ""
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.document_verification_event(case_id,purpose,subject_id,file_id,actor_kind,action,from_state,to_state,result_code,reason_code,revision) VALUES($1,$2,$3,$4,'scanner','scan_result',$5,$5,$6,NULLIF($7,''),$8)`, claim.CaseID, purpose, subject, claim.FileID, caseState, string(state), why, rev); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,last_error_code=NULLIF($2,''),updated_at=$3 WHERE id=$1`, claim.JobID, code, now)
		return err
	})
}

func (r *Repository) finishScan(ctx context.Context, claim ScanClaim, apply func(context.Context, pgx.Tx, time.Time) error) error {
	if r == nil || r.transactions == nil || !validUUID(claim.CaseID) || !validUUID(claim.FileID) || !validUUID(claim.JobID) || !validUUID(claim.SubjectID) || claim.CaseRevision <= 0 || claim.Attempt <= 0 || claim.LeaseUntil.IsZero() {
		return dv.ErrInvalid
	}
	if (claim.Purpose == dv.PurposeIdentity && claim.SpaceID != "") || (claim.Purpose == dv.PurposeSpace && !validUUID(claim.SpaceID)) || (claim.Purpose != dv.PurposeIdentity && claim.Purpose != dv.PurposeSpace) {
		return dv.ErrInvalid
	}
	tx, err := r.transactions.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var purpose dv.Purpose
	var subject, space string
	var rev int64
	var superseded *time.Time
	err = tx.QueryRow(ctx, `SELECT purpose,subject_id::text,COALESCE(space_id::text,''),revision,superseded_at FROM public.document_verification_case WHERE id=$1`, claim.CaseID).Scan(&purpose, &subject, &space, &rev, &nullablePT{v: &superseded})
	if err != nil {
		return err
	}
	if space != "" {
		owned, lockErr := lockSpace(ctx, tx, space, subject)
		if lockErr != nil {
			return lockErr
		}
		if !owned {
			return dv.ErrConflict
		}
	}
	if err = tx.QueryRow(ctx, `SELECT purpose,subject_id::text,COALESCE(space_id::text,''),revision,superseded_at FROM public.document_verification_case WHERE id=$1 FOR UPDATE`, claim.CaseID).Scan(&purpose, &subject, &space, &rev, &nullablePT{v: &superseded}); err != nil {
		return err
	}
	if purpose != claim.Purpose || subject != claim.SubjectID || space != claim.SpaceID || rev != claim.CaseRevision || superseded != nil {
		return dv.ErrConflict
	}
	var fileState string
	var fileRetentionAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT state,retention_at FROM public.document_verification_file WHERE id=$1 FOR UPDATE`, claim.FileID).Scan(&fileState, &nullablePT{v: &fileRetentionAt}); err != nil {
		return err
	}
	if fileState != "pendiente" || !sameRetentionDeadline(fileRetentionAt, claim.PriorRetentionAt) {
		return dv.ErrConflict
	}
	var state string
	var until time.Time
	err = tx.QueryRow(ctx, `SELECT state,lease_until FROM public.document_verification_job WHERE id=$1 AND file_id=$2 AND kind='scan' AND state='procesando' AND lease_until=$3 AND attempts=$4 FOR UPDATE`, claim.JobID, claim.FileID, claim.LeaseUntil, claim.Attempt).Scan(&state, &until)
	if errors.Is(err, pgx.ErrNoRows) {
		return dv.ErrConflict
	}
	if err != nil {
		return err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return err
	}
	if state != "procesando" || !until.Equal(claim.LeaseUntil) || !until.After(now) {
		return dv.ErrConflict
	}
	returnErr := apply(ctx, tx, now.UTC())
	if returnErr != nil {
		return returnErr
	}
	return tx.Commit(ctx)
}

func scanErrorCode(s string) bool {
	switch s {
	case "scanner_unavailable", "scanner_malformed", "signatures_stale", "malware_detected", "inspection_rejected", "retention_store_error":
		return true
	}
	return false
}

func scheduleScanRetention(ctx context.Context, tx pgx.Tx, fileID string, deadline time.Time) error {
	if !validUUID(fileID) || deadline.IsZero() {
		return dv.ErrInvalid
	}
	var completed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1
		FROM public.document_verification_job done
		WHERE done.file_id=$1
		  AND done.kind='retention_delete'
		  AND done.state='completado'
		  AND EXISTS (
		      SELECT 1
		      FROM public.document_verification_event e
		      WHERE e.file_id=done.file_id
		        AND e.action='retention_deleted'
		  )
	)`, fileID).Scan(&completed); err != nil {
		return err
	}
	if completed {
		_, err := tx.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,last_error_code=NULL,updated_at=clock_timestamp() WHERE file_id=$1 AND kind='retention_delete' AND state IN ('pendiente','procesando','reintentar')`, fileID)
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.document_verification_job(file_id,kind) VALUES($1,'retention_delete') ON CONFLICT (file_id,kind) WHERE state IN ('pendiente','procesando','reintentar') DO NOTHING`, fileID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE public.document_verification_job SET state='pendiente',attempts=0,available_at=$2,lease_until=NULL,last_error_code=NULL,updated_at=clock_timestamp() WHERE file_id=$1 AND kind='retention_delete' AND state IN ('pendiente','reintentar')`, fileID, deadline.UTC())
	return err
}

func cancelScanRetention(ctx context.Context, tx pgx.Tx, fileID string) error {
	if !validUUID(fileID) {
		return dv.ErrInvalid
	}
	_, err := tx.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,last_error_code=NULL,updated_at=clock_timestamp() WHERE file_id=$1 AND kind='retention_delete' AND state IN ('pendiente','procesando','reintentar')`, fileID)
	return err
}

func earlierRetentionDeadline(candidate time.Time, previous *time.Time) time.Time {
	if previous != nil && previous.Before(candidate) {
		return previous.UTC()
	}
	return candidate.UTC()
}

func sameRetentionDeadline(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func scanRetryRetentionPolicy(now time.Time, previousDeadline *time.Time, previousReason, previousError string, inspectedAt *time.Time) (time.Time, string) {
	if previousReason == "malicious_file" || previousError == "malware_detected" {
		malwareDeadline := now
		if inspectedAt != nil && !inspectedAt.After(now) {
			malwareDeadline = inspectedAt.Add(24 * time.Hour)
		}
		if previousDeadline != nil && previousDeadline.Before(malwareDeadline) {
			return previousDeadline.UTC(), previousReason
		}
		return malwareDeadline.UTC(), "malicious_file"
	}

	deadline := now.Add(7 * 24 * time.Hour)
	if previousDeadline != nil && !previousDeadline.After(deadline) && (previousDeadline.Before(deadline) || previousReason != "") {
		return previousDeadline.UTC(), previousReason
	}
	return deadline.UTC(), "inspection_error"
}

func normalizeScanLease(lease time.Duration) time.Duration {
	if lease < time.Millisecond {
		return time.Millisecond
	}
	return lease
}
