package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runLocalPrivacyCommand(command string, args []string) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return errors.New("invalid local API configuration")
	}
	pool, err := pgxpool.New(context.Background(), cfg.databaseURL())
	if err != nil {
		return errors.New("local database unavailable")
	}
	defer pool.Close()
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
		decoder := json.NewDecoder(io.LimitReader(os.Stdin, 2<<20))
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
		store, err := evidencefs.New(os.Getenv("M03_EVIDENCE_DIR"))
		if err != nil {
			return errors.New("local private storage unavailable")
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
