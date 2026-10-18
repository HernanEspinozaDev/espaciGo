package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
)

const localPrivacyReplayManifestMaxBytes = 16 << 20

func runLocalPrivacyCommand(command string, args []string) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return errors.New("invalid local API configuration")
	}
	pool, closeConnector, err := newDatabasePool(context.Background(), cfg)
	if err != nil {
		return errors.New("local database unavailable")
	}
	defer pool.Close()
	defer closeConnector()
	repo := identitypg.NewIdentityRepository(pool)
	service, err := privacy.NewService(repo)
	if err != nil {
		return errors.New("local privacy service unavailable")
	}
	var replayStore privacy.SuppressionReplayStore
	if command == "local-privacy-replay-export" || command == "local-privacy-replay-apply" {
		replayStore, err = newRuntimePrivacyReplayStore(context.Background(), os.Getenv)
		if err != nil {
			return errors.New("local privacy replay storage configuration is invalid")
		}
		if closer, ok := replayStore.(interface{ Close() error }); ok {
			defer closer.Close()
		}
	}
	switch command {
	case "local-privacy-retention-purge":
		store, err := evidencefs.New(os.Getenv("M03_EVIDENCE_DIR"))
		if err != nil {
			return errors.New("local private storage unavailable")
		}
		result, err := service.PurgeExpiredLocalRetentions(context.Background(), time.Now().UTC(), 100, store)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	case "local-privacy-replay-export":
		var manifest privacy.SuppressionReplayManifest
		if replayStore != nil {
			manifest, err = service.ExportSuppressionReplayManifestWithStore(context.Background(), replayStore)
		} else {
			manifest, err = service.ExportSuppressionReplayManifestWithFile(context.Background(), os.Getenv("LOCAL_PRIVACY_REPLAY_FILE"))
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(manifest)
	case "local-privacy-replay-apply":
		if len(args) != 2 {
			return errors.New("restore id and administrator id are required")
		}
		var manifest privacy.SuppressionReplayManifest
		input, err := io.ReadAll(io.LimitReader(os.Stdin, localPrivacyReplayManifestMaxBytes+1))
		if err != nil || len(input) > localPrivacyReplayManifestMaxBytes {
			return errors.New("invalid replay manifest")
		}
		decoder := json.NewDecoder(bytes.NewReader(input))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&manifest); err != nil {
			return errors.New("invalid replay manifest")
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return errors.New("invalid replay manifest")
		}
		if replayStore != nil {
			manifest, err = service.MergeSuppressionReplayManifestWithStore(context.Background(), manifest, replayStore)
			if err != nil {
				return errors.New("local privacy replay registry is unavailable or conflicted")
			}
		}
		store, err := newRuntimeFileStore(context.Background(), os.Getenv, os.Getenv("M03_EVIDENCE_DIR"))
		if err != nil {
			return errors.New("local private storage unavailable")
		}
		if closer, ok := store.(interface{ Close() error }); ok {
			defer closer.Close()
		}
		results, err := service.ReplaySuppressionManifest(context.Background(), manifest, args[0], args[1], time.Now().UTC(), store)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"restore_id": args[0], "results": results})
	default:
		return errors.New("unknown local privacy command")
	}
}
