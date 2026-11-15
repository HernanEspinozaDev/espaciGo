package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"strings"
	"time"
)

const (
	nativeV87Name        = "V000087__identity_declaration_retention_worker.sql"
	nativeV87Checksum    = "5eda043ddf3a3d5bb89e8d470eaf4e5450ed0964a9d59fe232bdcb37888b1d45"
	nativeV88Name        = "V000088__revoke_runtime_identity_purge_execute.sql"
	nativeV88Checksum    = "44cc5e74b8f8fcfba1633f01009024a7fdeae5683077b13677ed7bf2894083a8"
	nativeV89Name        = "V000089__identity_rut_claims.sql"
	nativeV89Checksum    = "93671b484f3118c697eebc4654f4b32325cce4231582b488f2e1a8d9b7fece9c"
	nativeV90Name        = "V000090__identity_verified_snapshot_purge.sql"
	nativeV90Checksum    = "23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592"
	nativeV91Name        = "V000091__identity_detached_terminal_withdrawal.sql"
	nativeV91Checksum    = "6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250"
	nativeV92Name        = "V000092__identity_expired_lifecycle_purge.sql"
	nativeV92Checksum    = "bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03"
	nativeV93Name        = "V000093__identity_verified_claim_reverification.sql"
	nativeV93Checksum    = "885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292"
	nativeV94Name        = "V000094__identity_delivered_session_outbox_retirement.sql"
	nativeV94Checksum    = "a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9"
	nativeV94Fingerprint = "395d771da8da79a6926feeeb43d9b355ea47778d311dffc1dd420aa63549eb4b"
	nativeV95Name        = "V000095__identity_document_case_declaration_binding.sql"
	nativeV95Checksum    = "351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275"
	nativeV95Fingerprint = "67077c1996cc72f2d6177d801fbfbbc445a53ba23b6306fc83ee18adfcbf3554"
	nativeV96Name        = "V000096__identity_reviewer_observation_audit.sql"
	nativeV96Checksum    = "a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5"
	nativeV96Fingerprint = "c71a06dfac444537bd0b05ef12346c57bcfda686c7761d53b52c24af25d02ab7"
)

func nativeKYCTailFingerprint(items []migrator.Migration, v int) (string, error) {
	if v < 87 || v > 96 {
		return "", errors.New("native tail supports V87 through V96")
	}
	if len(items) < v {
		return "", fmt.Errorf("inventory lacks V1..V%d", v)
	}
	h := sha256.New()
	for i, m := range items[:v] {
		if m.Version != int64(i+1) || m.Name == "" || len(m.Checksum) != 64 {
			return "", fmt.Errorf("invalid contiguous inventory at V%d", i+1)
		}
		fmt.Fprintf(h, "%d %s %s\n", m.Version, m.Name, m.Checksum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyNativeKYCTailLedgerFingerprint(entries []migrator.HistoryEntry, version int) error {
	items := make([]migrator.Migration, len(entries))
	for i, entry := range entries {
		items[i] = migrator.Migration{Version: entry.Version, Name: entry.Name, Checksum: entry.Checksum}
	}
	got, err := nativeKYCTailFingerprint(items, version)
	if err != nil {
		return err
	}
	want := nativeKYCTailReviewedFingerprint(version)
	if got != want {
		return fmt.Errorf("native KYC tail ledger V1..V%d differs from the reviewed source fingerprint", version)
	}
	return nil
}

func validateNativeKYCTailPlan(p migrator.PlanResult, v int) error {
	if v < 87 || v > 96 {
		return errors.New("unsupported native KYC tail version")
	}
	if p.Database != nativeAdminConfirmation || !p.ReadOnly || !p.HistoryExists || !strings.HasPrefix(strings.TrimSpace(p.ServerVersion), "18.7") {
		return errors.New("native tail target/ledger mismatch")
	}
	if len(p.Applied) != v-1 || len(p.Pending) != 1 {
		return fmt.Errorf("V%d requires exact preceding ledger prefix and one pending", v)
	}
	for i, e := range p.Applied {
		if e.Version != int64(i+1) || e.Name == "" || len(e.Checksum) != 64 {
			return fmt.Errorf("invalid applied ledger V%d", i+1)
		}
	}
	if v >= 95 {
		if err := verifyNativeKYCTailLedgerFingerprint(p.Applied, v-1); err != nil {
			return err
		}
		if p.IdentityDDLAuthority == nil {
			return fmt.Errorf("V%d requires the identity-tail DDL authority preflight", v)
		}
		if err := migrator.ValidateIdentityTailDDLAuthority(*p.IdentityDDLAuthority, int64(v)); err != nil {
			return err
		}
	}
	if v == 93 && (p.Applied[91].Name != nativeV92Name || p.Applied[91].Checksum != nativeV92Checksum) {
		return errors.New("V93 requires the exact pinned V92 predecessor")
	}
	if v == 94 && (p.Applied[92].Name != nativeV93Name || p.Applied[92].Checksum != nativeV93Checksum) {
		return errors.New("V94 requires the exact pinned V93 predecessor")
	}
	if v == 95 && (p.Applied[93].Name != nativeV94Name || p.Applied[93].Checksum != nativeV94Checksum) {
		return errors.New("V95 requires the exact pinned V94 predecessor")
	}
	if v == 96 && (p.Applied[94].Name != nativeV95Name || p.Applied[94].Checksum != nativeV95Checksum) {
		return errors.New("V96 requires the exact pinned V95 predecessor")
	}
	n, c := nativeV87Name, nativeV87Checksum
	if v == 88 {
		n, c = nativeV88Name, nativeV88Checksum
	} else if v == 89 {
		n, c = nativeV89Name, nativeV89Checksum
	} else if v == 90 {
		n, c = nativeV90Name, nativeV90Checksum
	} else if v == 91 {
		n, c = nativeV91Name, nativeV91Checksum
	} else if v == 92 {
		n, c = nativeV92Name, nativeV92Checksum
	} else if v == 93 {
		n, c = nativeV93Name, nativeV93Checksum
	} else if v == 94 {
		n, c = nativeV94Name, nativeV94Checksum
	} else if v == 95 {
		n, c = nativeV95Name, nativeV95Checksum
	} else if v == 96 {
		n, c = nativeV96Name, nativeV96Checksum
	}
	x := p.Pending[0]
	if x.Version != int64(v) || x.Name != n || x.Checksum != c {
		return errors.New("pending tail source differs from pinned migration")
	}
	return nil
}
func validateNativeKYCTailPost(p migrator.PlanResult, v int) error {
	if v < 87 || v > 96 || p.Database != nativeAdminConfirmation || !p.ReadOnly || !p.HistoryExists || !strings.HasPrefix(strings.TrimSpace(p.ServerVersion), "18.7") || len(p.Applied) != v || len(p.Pending) != 0 {
		return errors.New("native tail postcheck mismatch")
	}
	for i, e := range p.Applied {
		if e.Version != int64(i+1) || e.Name == "" || len(e.Checksum) != 64 {
			return fmt.Errorf("invalid post-ledger V%d", i+1)
		}
	}
	if v >= 95 {
		if err := verifyNativeKYCTailLedgerFingerprint(p.Applied, v); err != nil {
			return err
		}
	}
	name, checksum := nativeV87Name, nativeV87Checksum
	if v == 88 {
		name, checksum = nativeV88Name, nativeV88Checksum
	} else if v == 89 {
		name, checksum = nativeV89Name, nativeV89Checksum
	} else if v == 90 {
		name, checksum = nativeV90Name, nativeV90Checksum
	} else if v == 91 {
		name, checksum = nativeV91Name, nativeV91Checksum
	} else if v == 92 {
		name, checksum = nativeV92Name, nativeV92Checksum
	} else if v == 93 {
		name, checksum = nativeV93Name, nativeV93Checksum
	} else if v == 94 {
		name, checksum = nativeV94Name, nativeV94Checksum
	} else if v == 95 {
		name, checksum = nativeV95Name, nativeV95Checksum
	} else if v == 96 {
		name, checksum = nativeV96Name, nativeV96Checksum
	}
	if p.Applied[v-1].Name != name || p.Applied[v-1].Checksum != checksum {
		return errors.New("native KYC tail final ledger checksum mismatch")
	}
	return nil
}
func nativeKYCTailRequiresCreate(v int) bool {
	return migrator.NativeKYCMigrationRequiresCreate(int64(v))
}

func runNativeKYCTail(source string, v int, fp string, getenv func(string) string) error {
	s, e := loadConnectionSettings(getenv)
	if e != nil {
		return e
	}
	if e = validateNativeV86Connection(s, getenv); e != nil {
		return e
	}
	stage, cleanup, e := makeNativeKYCTailSnapshot(source, v, fp)
	if e != nil {
		return e
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = nativeV86IdentityPreflight(ctx, s.databaseURL, nativeKYCTailRequiresCreate(v)); e != nil {
		return e
	}
	p, e := migrator.Plan(ctx, s.databaseURL, stage, s.setRole)
	if e != nil {
		return e
	}
	if e = validateNativeKYCTailPlan(p, v); e != nil {
		return e
	}
	if v == 87 {
		_, e = migrator.RunNativeKYCTail87(ctx, s.databaseURL, stage)
	} else if v == 88 {
		_, e = migrator.RunNativeKYCTail88(ctx, s.databaseURL, stage)
	} else if v == 89 {
		_, e = migrator.RunNativeKYCTail89(ctx, s.databaseURL, stage)
	} else if v == 90 {
		_, e = migrator.RunNativeKYCTail90(ctx, s.databaseURL, stage)
	} else if v == 91 {
		_, e = migrator.RunNativeKYCTail91(ctx, s.databaseURL, stage)
	} else if v == 92 {
		_, e = migrator.RunNativeKYCTail92(ctx, s.databaseURL, stage)
	} else if v == 93 {
		_, e = migrator.RunNativeKYCTail93(ctx, s.databaseURL, stage)
	} else if v == 94 {
		_, e = migrator.RunNativeKYCTail94(ctx, s.databaseURL, stage)
	} else if v == 95 {
		_, e = migrator.RunNativeKYCTail95(ctx, s.databaseURL, stage)
	} else {
		_, e = migrator.RunNativeKYCTail96(ctx, s.databaseURL, stage)
	}
	if e != nil {
		return e
	}
	p, e = migrator.Plan(ctx, s.databaseURL, stage, s.setRole)
	if e != nil {
		return e
	}
	if e := validateNativeKYCTailPost(p, v); e != nil {
		return e
	}
	fmt.Printf("native V%d applied; source-backed ledger V1..V%d verified; later migrations not applied\n", v, v)
	return nil
}
