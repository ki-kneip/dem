package update

import (
	"errors"
	"testing"
)

func TestGuard(t *testing.T) {
	cases := []struct {
		name            string
		current, latest string
		allowMajor      bool
		wantErr         bool
		wantUpToDate    bool
	}{
		{name: "patch ok", current: "v1.2.3", latest: "v1.2.4"},
		{name: "minor ok", current: "1.2.3", latest: "1.3.0"},
		{name: "same version", current: "v1.2.3", latest: "v1.2.3", wantUpToDate: true},
		{name: "older remote", current: "v1.2.3", latest: "v1.0.0", wantUpToDate: true},
		{name: "major blocked", current: "v1.9.0", latest: "v2.0.0", wantErr: true},
		{name: "major allowed with flag", current: "v1.9.0", latest: "v2.0.0", allowMajor: true},
		{name: "dev build", current: "dev", latest: "v1.0.0", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Guard(tc.current, tc.latest, tc.allowMajor)
			switch {
			case tc.wantUpToDate:
				if !errors.Is(err, ErrUpToDate) {
					t.Fatalf("expected ErrUpToDate, got %v", err)
				}
			case tc.wantErr:
				if err == nil || errors.Is(err, ErrUpToDate) {
					t.Fatalf("expected a policy error, got %v", err)
				}
			default:
				if err != nil {
					t.Fatalf("expected the update to be allowed, got %v", err)
				}
			}
		})
	}
}
