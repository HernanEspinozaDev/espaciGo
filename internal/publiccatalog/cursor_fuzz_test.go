package publiccatalog

import "testing"

func FuzzDecodeCursorRejectsUnauthenticatedValues(f *testing.F) {
	service, err := New(memoryRepository{}, []byte("01234567890123456789012345678901"))
	if err != nil {
		f.Fatal(err)
	}
	want := Cursor{
		Version: 3, Fingerprint: "synthetic-filter-fingerprint", Sort: "default", PageSize: 20,
		LastSpaceID: "11111111-1111-4111-8111-111111111111",
	}
	valid, err := service.encodeCursor(want)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{valid, "", "v3..", "v3.e30.AA", "v2.e30.AA", valid + ".extra", "v3.$$.AA", "v3.e30.$$"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, err := service.decodeCursor(value)
		if err == nil && got != want {
			t.Fatalf("accepted unexpected decoded cursor %q as %#v", value, got)
		}
	})
}
