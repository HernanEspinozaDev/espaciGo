package identitydeclaration

import "testing"

const nativeAdapterTestsEnableEnv = "ESPACIGO_NATIVE_ADAPTER_TESTS"

func nativeTestDSNFromEnv(getenv func(string) string, dsnEnv string) string {
	if getenv(nativeAdapterTestsEnableEnv) != "1" {
		return ""
	}
	return getenv(dsnEnv)
}

func TestNativeResolversRequireExplicitOptIn(t *testing.T) {
	t.Setenv(nativeAdapterTestsEnableEnv, "")
	t.Setenv(rutClaimPostgresDSNEnv, "postgres://synthetic.invalid/rut")
	t.Setenv("IDENTITY_DECLARATION_NATIVE_TEST_DSN", "postgres://synthetic.invalid/runtime")
	if got := resolveRutClaimDSN(t); got != "" {
		t.Fatalf("resolveRutClaimDSN() returned a DSN without explicit opt-in")
	}
	if got := resolveRuntimeDSN(t); got != "" {
		t.Fatalf("resolveRuntimeDSN() returned a DSN without explicit opt-in")
	}
}

func TestNativeTestDSNRequiresExactGlobalOptInAndDedicatedDSN(t *testing.T) {
	const enabled = "1"
	const disabled = "0"
	const rutDSNEnv = "ESPACIGO_RUT_CLAIM_TEST_DATABASE_URL"
	const runtimeDSNEnv = "IDENTITY_DECLARATION_NATIVE_TEST_DSN"
	const syntheticDSN = "postgres://synthetic.invalid/test"

	cases := []struct {
		name   string
		env    map[string]string
		dsnEnv string
		want   string
	}{
		{name: "absent enable flag", env: map[string]string{rutDSNEnv: syntheticDSN}, dsnEnv: rutDSNEnv},
		{name: "missing dedicated DSN", env: map[string]string{"ESPACIGO_NATIVE_ADAPTER_TESTS": enabled}, dsnEnv: rutDSNEnv},
		{name: "wrong enable value zero", env: map[string]string{"ESPACIGO_NATIVE_ADAPTER_TESTS": disabled, rutDSNEnv: syntheticDSN}, dsnEnv: rutDSNEnv},
		{name: "wrong enable value true", env: map[string]string{"ESPACIGO_NATIVE_ADAPTER_TESTS": "true", rutDSNEnv: syntheticDSN}, dsnEnv: rutDSNEnv},
		{name: "exact explicit opt in", env: map[string]string{"ESPACIGO_NATIVE_ADAPTER_TESTS": enabled, rutDSNEnv: syntheticDSN}, dsnEnv: rutDSNEnv, want: syntheticDSN},
		{name: "other test DSN cannot enable", env: map[string]string{"ESPACIGO_NATIVE_ADAPTER_TESTS": enabled, rutDSNEnv: syntheticDSN}, dsnEnv: runtimeDSNEnv},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nativeTestDSNFromEnv(func(key string) string { return tc.env[key] }, tc.dsnEnv)
			if got != tc.want {
				t.Fatalf("nativeTestDSNFromEnv() = %q, want %q", got, tc.want)
			}
		})
	}
}
