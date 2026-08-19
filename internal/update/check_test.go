package update

import (
	"path/filepath"
	"strings"
	"testing"
)

// state points the check at a temp directory, so a test never reads or writes
// the real one.
func state(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv(DisableEnv, "")
}

func TestNoticeMentionsANewerRelease(t *testing.T) {
	state(t)
	serve(t, "0.2.0", "new binary", intact)

	notice := Notice(t.Context(), "0.1.1")
	if !strings.Contains(notice, "0.2.0") {
		t.Fatalf("notice = %q, want it to mention 0.2.0", notice)
	}

	// The lookup happened once; what follows is answered from the record.
	for i := range notifyLimit - 1 {
		if notice := Notice(t.Context(), "0.1.1"); notice == "" {
			t.Fatalf("notice %d = %q, want a notice", i+2, notice)
		}
	}

	// And then it stops: a release mentioned three times is not news.
	if notice := Notice(t.Context(), "0.1.1"); notice != "" {
		t.Errorf("notice %d = %q, want none", notifyLimit+1, notice)
	}
}

func TestNoticeSaysNothingWhenCurrent(t *testing.T) {
	state(t)
	serve(t, "0.2.0", "new binary", intact)

	if notice := Notice(t.Context(), "0.2.0"); notice != "" {
		t.Errorf("notice = %q, want none", notice)
	}
	if notice := Notice(t.Context(), "0.3.0"); notice != "" {
		t.Errorf("notice for a build ahead of the release = %q, want none", notice)
	}
}

func TestNoticeSaysNothingForASourceBuild(t *testing.T) {
	state(t)
	serve(t, "0.2.0", "new binary", intact)

	if notice := Notice(t.Context(), "dev"); notice != "" {
		t.Errorf("notice = %q, want none", notice)
	}
}

func TestNoticeCanBeTurnedOff(t *testing.T) {
	state(t)
	t.Setenv(DisableEnv, "1")
	// No server: the check must not reach the network at all, and a request
	// to github would fail the test by making it slow rather than red.
	if notice := Notice(t.Context(), "0.1.1"); notice != "" {
		t.Errorf("notice = %q, want none", notice)
	}
}

// A network that is not there is not an error anyone should hear about.
func TestNoticeSurvivesAFailedLookup(t *testing.T) {
	state(t)
	serve(t, "0.2.0", "new binary", intact)
	latestURL = "http://127.0.0.1:1/latest"

	if notice := Notice(t.Context(), "0.1.1"); notice != "" {
		t.Errorf("notice = %q, want none", notice)
	}
}
