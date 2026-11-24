package model

import "testing"

func TestRequestOptionsResolveNoTimeout(t *testing.T) {
	s := DefaultSettings()
	s.RequestTimeoutSec = NoTimeout

	inherited := (RequestOptions{}).Resolve(s)
	if inherited.TimeoutMs != NoTimeout {
		t.Fatalf("inherited TimeoutMs = %d, want %d", inherited.TimeoutMs, NoTimeout)
	}

	explicit := (RequestOptions{TimeoutMs: 2500}).Resolve(s)
	if explicit.TimeoutMs != 2500 {
		t.Fatalf("explicit TimeoutMs = %d, want 2500", explicit.TimeoutMs)
	}

	legacy := (RequestOptions{}).Resolve(Settings{RequestTimeoutSec: 0})
	if legacy.TimeoutMs != 60_000 {
		t.Fatalf("legacy TimeoutMs = %d, want 60000", legacy.TimeoutMs)
	}
}
