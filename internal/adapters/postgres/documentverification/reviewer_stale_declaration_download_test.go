package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

const staleReviewerDownloadFile = "66666666-6666-4666-8666-666666666666"

type staleReviewerDownloadTx struct {
	*staleReviewerReadTx
	fileRows [][]any
}

func (tx *staleReviewerDownloadTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if bytes.Contains([]byte(query), []byte("FROM public.document_verification_file")) {
		tx.fileQueryCalls++
		return &reviewerAuditFakeRows{rows: tx.fileRows}, nil
	}
	return tx.staleReviewerReadTx.Query(ctx, query, args...)
}

type staleReviewerDownloadStore struct {
	content      []byte
	identityRead int
	spaceRead    int
	subject      string
	caseID       string
	fileID       string
}

func (s *staleReviewerDownloadStore) ReadIdentityDocument(_ context.Context, subject, caseID, fileID string) ([]byte, error) {
	s.identityRead++
	s.subject, s.caseID, s.fileID = subject, caseID, fileID
	return append([]byte(nil), s.content...), nil
}

func (s *staleReviewerDownloadStore) ReadSpaceDocument(context.Context, string, string, string, string) ([]byte, error) {
	s.spaceRead++
	return nil, errors.New("unexpected space document read")
}

func TestReviewerDownloadRejectsStaleDeclarationBeforeFileOrStoreRead(t *testing.T) {
	content := []byte("synthetic identity evidence")
	digest := sha256.Sum256(content)
	for _, tc := range []struct {
		name       string
		latestID   string
		wantErr    error
		wantReads  int
		wantCommit bool
	}{
		{
			name:     "stale declaration hides evidence",
			latestID: staleReviewerReadLatest,
			wantErr:  dv.ErrNotFound,
		},
		{
			name:       "current declaration permits assigned clean evidence",
			latestID:   staleReviewerReadDeclaration,
			wantReads:  1,
			wantCommit: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseTx := &staleReviewerReadTx{
				reviewerAuditFakeTx: &reviewerAuditFakeTx{
					actor: staleReviewerReadActor, subject: staleReviewerReadSubject,
					caseID: staleReviewerReadCase, declarationID: staleReviewerReadDeclaration,
					consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
				},
				latestDeclarationID: tc.latestID,
			}
			tx := &staleReviewerDownloadTx{
				staleReviewerReadTx: baseTx,
				fileRows: [][]any{{
					staleReviewerDownloadFile, string(dv.FileClean), int64(len(content)), "image/png", int32(1), digest[:], nil,
				}},
			}
			repo := newWithTransactionBeginner(staleReviewerReadBeginner{tx: tx})
			cases, err := dv.NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			store := &staleReviewerDownloadStore{content: content}
			access, err := dv.NewReviewerAccessService(repo, cases, store)
			if err != nil {
				t.Fatal(err)
			}

			got, err := access.Download(context.Background(), staleReviewerReadActor, staleReviewerReadCase, staleReviewerDownloadFile)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Download error=%v, want %v", err, tc.wantErr)
			}
			if store.identityRead != tc.wantReads || store.spaceRead != 0 {
				t.Fatalf("store reads identity=%d space=%d, want %d/0", store.identityRead, store.spaceRead, tc.wantReads)
			}
			if tc.wantReads == 0 {
				if len(got.Content) != 0 || got.MIME != "" {
					t.Fatalf("stale download leaked result: MIME=%q bytes=%d", got.MIME, len(got.Content))
				}
			} else {
				if got.MIME != "image/png" || !bytes.Equal(got.Content, content) {
					t.Fatalf("current download=%+v", got)
				}
				if store.subject != staleReviewerReadSubject || store.caseID != staleReviewerReadCase || store.fileID != staleReviewerDownloadFile {
					t.Fatalf("store scope=%q/%q/%q", store.subject, store.caseID, store.fileID)
				}
			}
			if tx.currentQueryCalls != 1 || tx.fileQueryCalls != tc.wantReads || tx.committed != tc.wantCommit || len(tx.execs) != 0 {
				t.Fatalf("freshness checks=%d file metadata reads=%d committed=%v writes=%d", tx.currentQueryCalls, tx.fileQueryCalls, tx.committed, len(tx.execs))
			}
		})
	}
}
