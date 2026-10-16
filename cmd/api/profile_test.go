package main

import "testing"

func TestAuthPrototypeProfileRequiresExplicitCloudTestOrLocalFlag(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]string
		want   bool
		bad    bool
	}{
		{name: "disabled by default", values: map[string]string{}, want: false},
		{name: "existing local flag", values: map[string]string{"LOCAL_AUTH_PROTOTYPE": "1"}, want: true},
		{name: "explicit cloud test", values: map[string]string{"APP_PROFILE": "cloud-test"}, want: true},
		{name: "no production alias", values: map[string]string{"APP_PROFILE": "production"}, bad: true},
		{name: "no ambiguous mixed flags", values: map[string]string{"APP_PROFILE": "cloud-test", "LOCAL_AUTH_PROTOTYPE": "1"}, bad: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := authPrototypeEnabled(func(key string) string { return test.values[key] })
			if (err != nil) != test.bad || (!test.bad && got != test.want) {
				t.Fatalf("enabled=%v err=%v", got, err)
			}
		})
	}
}
