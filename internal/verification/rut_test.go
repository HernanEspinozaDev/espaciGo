package verification

import "testing"

func TestNormalizeRut(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"12.345.678-5", "12345678-5"}, {"1000005-k", "1000005-K"}, {"12345678-5", "12345678-5"}} {
		got, err := NormalizeRut(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeRut(%q)=(%q,%v), want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "12.345.678-4", "12345678-", "12345678-A", "12.345.6785", "123456789-5", "-5"} {
		if ValidateRut(in) {
			t.Errorf("ValidateRut(%q)=true", in)
		}
	}
}
