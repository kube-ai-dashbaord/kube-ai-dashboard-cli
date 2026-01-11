package i18n

import "testing"

func TestTranslation(t *testing.T) {
	SetLanguage("en")
	if T("app_title") != "k13s - K8s AI Explorer" {
		t.Errorf("expected English title, got %s", T("app_title"))
	}

	SetLanguage("ko")
	if T("app_title") != "k13s - K8s AI 탐색기" {
		t.Errorf("expected Korean title, got %s", T("app_title"))
	}

	// Fallback test
	SetLanguage("non-existent")
	if T("app_title") != "k13s - K8s AI Explorer" {
		t.Errorf("expected fallback to English title, got %s", T("app_title"))
	}
}
