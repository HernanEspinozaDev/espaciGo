package documentverification

import "time"

// AuthorizeSpaceOwnerDownload enforces the owner-side binary download policy
// for an already loaded, authoritative case snapshot. The caller must obtain
// both the snapshot and Authorization from trusted repository state; this
// predicate does not load or return file bytes and grants no reviewer/support
// access.
func AuthorizeSpaceOwnerDownload(x Snapshot, a Authorization, actor, fileID string) error {
	return AuthorizeSpaceOwnerDownloadAt(x, a, actor, fileID, time.Now().UTC())
}

// AuthorizeSpaceOwnerDownloadAt makes retention-boundary checks deterministic
// for callers and tests that already have an authoritative current time.
func AuthorizeSpaceOwnerDownloadAt(x Snapshot, a Authorization, actor, fileID string, now time.Time) error {
	if ValidateID(actor) != nil || ValidateID(fileID) != nil || !validSnapshot(x) {
		return ErrInvalid
	}
	if now.IsZero() {
		return ErrInvalid
	}
	if x.Purpose != PurposeSpace || x.SupersededAt != nil || a.ActorID != actor || ValidateID(a.SpaceOwnerID) != nil || a.SpaceOwnerID != actor || x.SubjectID != a.SpaceOwnerID {
		return ErrCapability
	}
	switch x.State {
	case CaseDraft, CaseSubmitted, CaseInReview, CaseNeedsEvidence:
		// Open cases allow the current owner to retrieve clean files only.
	default:
		return ErrTransition
	}
	for _, f := range x.Files {
		if f.ID != fileID {
			continue
		}
		if !f.RetentionAt.IsZero() && !f.RetentionAt.After(now) {
			return ErrNotFound
		}
		if f.State != FileClean {
			return ErrFilesNotClean
		}
		return nil
	}
	return ErrNotFound
}
