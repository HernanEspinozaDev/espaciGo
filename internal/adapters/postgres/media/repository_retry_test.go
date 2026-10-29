package mediapg

import "testing"

func TestCanRetryFailedReservation(t *testing.T) {
	tests := []struct {
		name, state                  string
		hashMatches, artifacts, want bool
	}{
		{"failed no artifacts", "failed", true, false, true},
		{"hash mismatch", "failed", false, false, false},
		{"variant artifact", "failed", true, true, false},
		{"pending", "pending", true, false, false},
		{"deleting", "deleting", true, false, false},
		{"deleted", "deleted", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canRetryFailed(tt.state, tt.hashMatches, tt.artifacts); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
