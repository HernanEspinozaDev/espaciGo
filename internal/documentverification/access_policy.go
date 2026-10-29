package documentverification

// AuthorizeSpaceOwnerDownload enforces the owner-side binary download policy
// for an already loaded, authoritative case snapshot. The caller must obtain
// both the snapshot and Authorization from trusted repository state; this
// predicate does not load or return file bytes and grants no reviewer/support
// access.
func AuthorizeSpaceOwnerDownload(x Snapshot, a Authorization, actor, fileID string) error {
	if ValidateID(actor) != nil || ValidateID(fileID) != nil || !validSnapshot(x) {
		return ErrInvalid
	}
	if x.Purpose != PurposeSpace || a.ActorID != actor || ValidateID(a.SpaceOwnerID) != nil || a.SpaceOwnerID != actor || x.SubjectID != a.SpaceOwnerID {
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
		if f.State != FileClean {
			return ErrFilesNotClean
		}
		return nil
	}
	return ErrNotFound
}
