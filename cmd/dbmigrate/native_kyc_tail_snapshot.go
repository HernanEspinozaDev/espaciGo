package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
)

func nativeKYCTailReviewedFingerprint(v int) string {
	switch v {
	case 87:
		return "4ad03308b1a8e24c611c4f3327ac7c44b0f6e7793e5905db45d87bddfb78375f"
	case 88:
		return "8a88aea304ce45e34128017b61c56ab3b517b896a1a119a64d7f93abee2967bb"
	case 89:
		return "94d2f333ee0d5649f54821ac4094e14e4df32cc2cf8c6b4c7d7a4cfe9dca30e5"
	case 90:
		return "9f682b299547fdd3ac97306cbe6b4804fd121b0dbf827d95aa4f754f9c4cbd81"
	case 91:
		return "40a36dd98b9dc916ab1bb106da1c77c67f11a9a67de3b69140faf252e1462814"
	case 92:
		return "a49c75125218df000104597604c86c95df94efd9aaf3f584392488307cc23f4e"
	case 93:
		return "366b7accc2c15c62d66f5cecc8b36a41addc241ede68fd2febf291adadd3073e"
	case 94:
		return nativeV94Fingerprint
	case 95:
		return nativeV95Fingerprint
	case 96:
		return nativeV96Fingerprint
	default:
		return ""
	}
}
func validateNativeKYCTailMode(v int, confirmation, fingerprint string, flags []string) error {
	if v < 87 || v > 96 || confirmation != nativeAdminConfirmation || fingerprint != nativeKYCTailReviewedFingerprint(v) {
		return errors.New("native KYC tail requires version 87 through 96, exact target confirmation and pinned source fingerprint")
	}
	for _, name := range flags {
		switch name {
		case "native-kyc-tail-version", "confirm-native-kyc-tail", "native-kyc-tail-source-sha256", "dir":
		default:
			return errors.New("native KYC tail cannot combine with another mode or unrelated option")
		}
	}
	return nil
}
func makeNativeKYCTailSnapshot(source string, v int, fingerprint string) (string, func(), error) {
	if nativeKYCTailReviewedFingerprint(v) == "" || fingerprint != nativeKYCTailReviewedFingerprint(v) {
		return "", nil, errors.New("native KYC tail unreviewed version or fingerprint")
	}
	items, err := migrator.DiscoverMigrations(source)
	if err != nil {
		return "", nil, err
	}
	actual, err := nativeKYCTailFingerprint(items, v)
	if err != nil || actual != fingerprint {
		return "", nil, errors.New("native KYC tail source differs from reviewed fingerprint")
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
	if items[v-1].Name != name || items[v-1].Checksum != checksum {
		return "", nil, errors.New("native KYC tail migration checksum mismatch")
	}
	stage, err := os.MkdirTemp("", "native-kyc-tail-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(stage) }
	for _, m := range items[:v] {
		if err := os.WriteFile(filepath.Join(stage, m.Name), m.SQL, 0600); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	staged, err := migrator.DiscoverMigrations(stage)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	stagedFP, err := nativeKYCTailFingerprint(staged, v)
	if err != nil || len(staged) != v || stagedFP != fingerprint {
		cleanup()
		return "", nil, errors.New("native KYC tail staged source mismatch")
	}
	return stage, cleanup, nil
}
