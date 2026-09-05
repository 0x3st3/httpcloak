package client

import "testing"

func TestJoinURLPreservesRedirectQuery(t *testing.T) {
	got := JoinURL(
		"https://www.micromania.fr/on/demandware.store/Sites-Micromania-Site/fr_FR/Payline-Return?paylinetoken=payment-token",
		"/stdcheckout/standard/confirmation?ID=261048396&token=confirmation-token",
	)
	want := "https://www.micromania.fr/stdcheckout/standard/confirmation?ID=261048396&token=confirmation-token"
	if got != want {
		t.Fatalf("redirect URL = %q, want %q", got, want)
	}
}
