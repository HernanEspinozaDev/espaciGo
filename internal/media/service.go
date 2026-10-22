package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

type IDGenerator interface{ ID() (string, error) }

type UploadResult struct {
	Photo   Photo
	Reused  bool
	Pending bool
}

type Service struct {
	repo         Repository
	store        ObjectStore
	ids          IDGenerator
	now          func() time.Time
	convertSlots chan struct{}
}

func NewService(repo Repository, store ObjectStore, ids IDGenerator, now func() time.Time) (*Service, error) {
	if repo == nil || store == nil || ids == nil || now == nil {
		return nil, ErrInvalid
	}
	return &Service{repo: repo, store: store, ids: ids, now: now, convertSlots: make(chan struct{}, 2)}, nil
}

func (s *Service) UploadSpace(ctx context.Context, owner, spaceID, key string, raw []byte) (UploadResult, error) {
	return s.upload(ctx, owner, spaceID, PurposeSpace, key, raw)
}

func (s *Service) UploadProfile(ctx context.Context, owner, key string, raw []byte) (UploadResult, error) {
	return s.upload(ctx, owner, "", PurposeProfile, key, raw)
}

func (s *Service) upload(ctx context.Context, owner, spaceID string, purpose Purpose, key string, raw []byte) (UploadResult, error) {
	key = strings.TrimSpace(key)
	if !validUUID(owner) || (purpose == PurposeSpace && !validUUID(spaceID)) || len(key) < 8 || len(key) > 128 {
		return UploadResult{}, ErrInvalid
	}
	if _, _, err := ValidateInput(raw); err != nil {
		return UploadResult{}, err
	}
	digest := sha256.Sum256(raw)
	inputHash := hex.EncodeToString(digest[:])
	assetID, err := s.ids.ID()
	if err != nil || !validUUID(assetID) {
		return UploadResult{}, ErrUnavailable
	}
	at := s.now().UTC()
	reservation, reused, err := s.repo.Reserve(ctx, assetID, owner, spaceID, purpose, key, inputHash, at, at.Add(15*time.Minute))
	if err != nil {
		return UploadResult{}, mapRepositoryError(err)
	}
	if reused {
		if reservation.InputHash != inputHash {
			return UploadResult{}, ErrConflict
		}
		if reservation.State == "ready" {
			photo, err := s.photoByPurpose(ctx, owner, spaceID, purpose, reservation.ID)
			if err != nil {
				return UploadResult{}, err
			}
			return UploadResult{Photo: photo, Reused: true}, nil
		}
		if reservation.State == "pending" && reservation.LeaseTill.After(at) {
			return UploadResult{Photo: Photo{ID: reservation.ID, SpaceID: spaceID, State: "pending"}, Reused: true, Pending: true}, nil
		}
		return UploadResult{}, ErrConflict
	}
	select {
	case s.convertSlots <- struct{}{}:
	case <-ctx.Done():
		_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
		return UploadResult{}, ctx.Err()
	}
	defer func() { <-s.convertSlots }()
	variants, err := Convert(raw, purpose)
	if err != nil {
		_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
		return UploadResult{}, err
	}
	for i := range variants {
		key := objectKey(purpose, owner, spaceID, reservation.ID, reservation.Token, variants[i].Name)
		if err = s.repo.PlanVariant(ctx, reservation.ID, reservation.Token, variants[i].Name, key); err != nil {
			_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
			return UploadResult{}, mapRepositoryError(err)
		}
		stored, putErr := s.store.Put(ctx, key, variants[i].Bytes, variants[i].MIME)
		if putErr != nil {
			// Mark this exact write terminal even if cleanup already fenced the
			// asset. Cleanup still resolves Generation to catch ambiguous writes.
			_ = s.repo.FinishVariantWrite(context.Background(), reservation.ID, variants[i].Name, key)
			_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
			return UploadResult{}, ErrUnavailable
		}
		if err = s.repo.FinishVariantWrite(ctx, reservation.ID, variants[i].Name, key); err != nil {
			_ = s.store.Delete(context.Background(), stored.ObjectKey, stored.Generation)
			_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
			return UploadResult{}, ErrUnavailable
		}
		variants[i].ObjectKey = stored.ObjectKey
		variants[i].Provider = stored.Provider
		variants[i].Bucket = stored.Bucket
		variants[i].Generation = stored.Generation
		variants[i].Size = stored.Size
		variants[i].SHA256 = stored.SHA256
		if err = s.repo.RecordVariant(ctx, reservation.ID, reservation.Token, variants[i]); err != nil {
			_ = s.store.Delete(context.Background(), stored.ObjectKey, stored.Generation)
			_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
			return UploadResult{}, ErrUnavailable
		}
	}
	if purpose == PurposeSpace {
		photo, err := s.repo.FinalizeSpace(ctx, reservation.ID, reservation.Token, owner, spaceID, s.now().UTC())
		if err != nil {
			_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
			return UploadResult{}, mapRepositoryError(err)
		}
		return UploadResult{Photo: s.spaceURLs(photo)}, nil
	}
	if err = s.repo.FinalizeProfile(ctx, owner, reservation.ID, reservation.Token, s.now().UTC()); err != nil {
		_ = s.repo.FailReservation(context.Background(), reservation.ID, reservation.Token, s.now().UTC())
		return UploadResult{}, mapRepositoryError(err)
	}
	photo, err := s.repo.GetProfilePhoto(ctx, owner)
	if err != nil {
		return UploadResult{}, mapRepositoryError(err)
	}
	return UploadResult{Photo: s.profileURL(photo)}, nil
}

func (s *Service) photoByPurpose(ctx context.Context, owner, spaceID string, purpose Purpose, id string) (Photo, error) {
	if purpose == PurposeSpace {
		photo, err := s.repo.GetSpace(ctx, owner, spaceID, id)
		return s.spaceURLs(photo), err
	}
	photo, err := s.repo.GetProfilePhoto(ctx, owner)
	return s.profileURL(photo), err
}

func (s *Service) ListSpace(ctx context.Context, owner, spaceID string) ([]Photo, error) {
	if !validUUID(owner) || !validUUID(spaceID) {
		return nil, ErrNotFound
	}
	items, err := s.repo.ListSpace(ctx, owner, spaceID)
	for i := range items {
		items[i] = s.spaceURLs(items[i])
	}
	return items, err
}

func (s *Service) GetSpace(ctx context.Context, owner, spaceID, photoID string) (Photo, error) {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return Photo{}, ErrNotFound
	}
	photo, err := s.repo.GetSpace(ctx, owner, spaceID, photoID)
	return s.spaceURLs(photo), err
}

func (s *Service) UpdateSpace(ctx context.Context, owner, spaceID, photoID string, alt *string, visible *bool, position *int) (Photo, error) {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) || (alt != nil && len([]rune(*alt)) > MaxAltText) || (position != nil && (*position < 1 || *position > MaxSpaceMedia)) {
		return Photo{}, ErrInvalid
	}
	photo, err := s.repo.UpdateSpacePhoto(ctx, owner, spaceID, photoID, alt, visible, position, s.now().UTC())
	return s.spaceURLs(photo), err
}

func (s *Service) RemoveSpace(ctx context.Context, owner, spaceID, photoID string) error {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return ErrNotFound
	}
	return mapRepositoryError(s.repo.RemoveSpacePhoto(ctx, owner, spaceID, photoID, s.now().UTC()))
}

func (s *Service) SelectCover(ctx context.Context, owner, spaceID, photoID string) error {
	if !validUUID(owner) || !validUUID(spaceID) || !validUUID(photoID) {
		return ErrNotFound
	}
	return mapRepositoryError(s.repo.SelectSpaceCover(ctx, owner, spaceID, photoID, s.now().UTC()))
}

func (s *Service) ContentSpace(ctx context.Context, owner, spaceID, photoID string, name VariantName) (Photo, Variant, []byte, error) {
	photo, err := s.GetSpace(ctx, owner, spaceID, photoID)
	if err != nil {
		return Photo{}, Variant{}, nil, err
	}
	variant, err := s.repo.GetVariant(ctx, owner, photoID, string(name))
	if err != nil {
		return Photo{}, Variant{}, nil, mapRepositoryError(err)
	}
	b, err := s.store.Get(ctx, variant.ObjectKey, variant.Generation)
	if err != nil {
		return Photo{}, Variant{}, nil, ErrUnavailable
	}
	if int64(len(b)) != variant.Size || digestHex(b) != variant.SHA256 {
		return Photo{}, Variant{}, nil, ErrUnavailable
	}
	return photo, variant, b, nil
}

func (s *Service) RemoveProfile(ctx context.Context, owner string) error {
	if !validUUID(owner) {
		return ErrNotFound
	}
	return mapRepositoryError(s.repo.DeleteProfilePhoto(ctx, owner, s.now().UTC()))
}

func (s *Service) ProfileContent(ctx context.Context, owner string) (Photo, Variant, []byte, error) {
	if !validUUID(owner) {
		return Photo{}, Variant{}, nil, ErrNotFound
	}
	p, err := s.repo.GetProfilePhoto(ctx, owner)
	if err != nil {
		return Photo{}, Variant{}, nil, mapRepositoryError(err)
	}
	if len(p.Variants) != 1 {
		return Photo{}, Variant{}, nil, ErrNotFound
	}
	p = s.profileURL(p)
	v := p.Variants[0]
	b, err := s.store.Get(ctx, v.ObjectKey, v.Generation)
	if err != nil {
		return Photo{}, Variant{}, nil, ErrUnavailable
	}
	if int64(len(b)) != v.Size || digestHex(b) != v.SHA256 {
		return Photo{}, Variant{}, nil, ErrUnavailable
	}
	return p, v, b, nil
}

func (s *Service) spaceURLs(p Photo) Photo {
	for i := range p.Variants {
		p.Variants[i].URL = "/api/v1/spaces/" + p.SpaceID + "/gallery/" + p.ID + "/variants/" + string(p.Variants[i].Name)
	}
	return p
}

func (s *Service) profileURL(p Photo) Photo {
	for i := range p.Variants {
		p.Variants[i].URL = "/api/v1/profile/photo/content"
	}
	return p
}

func (s *Service) CleanOnce(ctx context.Context, limit int) error {
	if limit < 1 || limit > 500 {
		return ErrInvalid
	}
	items, err := s.repo.ClaimCleanup(ctx, limit, s.now().UTC())
	if err != nil {
		return err
	}
	for _, item := range items {
		failed := false
		for _, intent := range item.Intents {
			generation := intent.Generation
			if generation == 0 {
				var resolveErr error
				generation, resolveErr = s.store.Generation(ctx, intent.ObjectKey)
				if errors.Is(resolveErr, ErrNotFound) {
					// An active writer can still make this key appear. Retain its
					// intent until that writer reports completion.
					if !intent.WriteComplete {
						failed = true
					}
					continue
				}
				if resolveErr != nil {
					failed = true
					continue
				}
			}
			if e := s.store.Delete(ctx, intent.ObjectKey, generation); e != nil {
				failed = true
			}
		}
		for _, v := range item.Variants {
			if e := s.store.Delete(ctx, v.ObjectKey, v.Generation); e != nil {
				failed = true
			}
		}
		if failed {
			if e := s.repo.FailCleanup(ctx, item.AssetID, item.Token, s.now().UTC()); e != nil {
				return e
			}
			continue
		}
		if err := s.repo.CompleteCleanup(ctx, item.AssetID, item.Token, s.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func objectKey(p Purpose, owner, spaceID, assetID string, token int64, name VariantName) string {
	if p == PurposeProfile {
		return "media/v1/profiles/" + owner + "/" + assetID + "/" + strconv.FormatInt(token, 10) + "/" + string(name) + ".webp"
	}
	return "media/v1/spaces/" + spaceID + "/" + assetID + "/" + strconv.FormatInt(token, 10) + "/" + string(name) + ".webp"
}

func digestHex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvalid) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrLimit) || errors.Is(err, ErrConflict) {
		return err
	}
	return err
}
