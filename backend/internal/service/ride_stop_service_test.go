package service

import "testing"

func TestStopDetourExceedsLimits(t *testing.T) {
	tests := []struct {
		name             string
		baseDistanceKM   float64
		baseDurationSecs float64
		proposedDistance float64
		proposedDuration float64
		wantExceeded     bool
	}{
		{name: "within both limits", baseDistanceKM: 10, baseDurationSecs: 900, proposedDistance: 12, proposedDuration: 1500},
		{name: "distance exceeds limit", baseDistanceKM: 10, baseDurationSecs: 900, proposedDistance: 12.01, proposedDuration: 900, wantExceeded: true},
		{name: "time exceeds limit", baseDistanceKM: 10, baseDurationSecs: 900, proposedDistance: 10, proposedDuration: 1501, wantExceeded: true},
		{name: "either threshold rejects", baseDistanceKM: 10, baseDurationSecs: 900, proposedDistance: 12.01, proposedDuration: 1501, wantExceeded: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stopDetourExceedsLimits(test.baseDistanceKM, test.baseDurationSecs, test.proposedDistance, test.proposedDuration)
			if got != test.wantExceeded {
				t.Fatalf("stopDetourExceedsLimits() = %v, want %v", got, test.wantExceeded)
			}
		})
	}
}
