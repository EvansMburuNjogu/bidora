package identity_test

import (
	"bidora/backend/internal/identity/browser"
	"testing"
)

func TestAuditBrowserNameIsBoundedAndDoesNotExposeRawUserAgent(t *testing.T) {
	for agent, want := range map[string]string{
		"": "", "Chrome": "Chrome", "Mozilla/5.0 Chrome/123 Safari/537.36": "Chrome", "Mozilla/5.0 Chrome/123 Safari/537.36 Edg/123": "Edge", "Mozilla/5.0 Chrome/123 OPR/100": "Opera", "Mozilla/5.0 FxiOS/100 Safari/600": "Firefox", "Mozilla/5.0 Version/17 Safari/605": "Safari", "private-user-supplied-text": "Unknown browser",
	} {
		if got := browser.Name(agent); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
