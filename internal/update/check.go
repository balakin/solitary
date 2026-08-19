package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/balakin/solitary/internal/config"
)

const (
	// interval is how often github is asked. A check costs a round trip, so
	// it happens once a day rather than once a command.
	interval = 24 * time.Hour

	// notifyLimit is how often one release is mentioned before it is dropped.
	// A notice nobody acted on three times is noise.
	notifyLimit = 3

	// checkTimeout keeps an unreachable network from holding up a command
	// that has nothing to do with the network.
	checkTimeout = 2 * time.Second
)

// DisableEnv turns the check off entirely, for anyone who would rather solitary
// never talked to github on its own.
const DisableEnv = "SOLITARY_NO_UPDATE_CHECK"

// record is what a check leaves behind between runs. It lives in the state
// directory: it is generated, and nothing generated goes where cell
// definitions are edited by hand.
type record struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
	Notified  string    `json:"notified"`
	Count     int       `json:"count"`
}

// Notice is the line to print about a newer release, or "" when there is
// nothing to say. Every failure along the way is one of those: a version check
// is not a reason for a command to fail, or to say anything about itself.
func Notice(ctx context.Context, current string) string {
	if os.Getenv(DisableEnv) != "" {
		return ""
	}
	// A source build has no release to be behind.
	if !IsRelease(current) {
		return ""
	}

	path, err := recordFile()
	if err != nil {
		return ""
	}

	saved := read(path)
	if time.Since(saved.CheckedAt) >= interval {
		ctx, cancel := context.WithTimeout(ctx, checkTimeout)
		defer cancel()

		// The time is recorded even when the lookup failed, so that a
		// machine with no network pays for one attempt a day rather than
		// one per command.
		saved.CheckedAt = time.Now()
		if latest, err := Latest(ctx); err == nil {
			saved.Latest = latest
		}
	}

	if !Newer(current, saved.Latest) {
		write(path, saved)
		return ""
	}

	if saved.Notified != saved.Latest {
		saved.Notified, saved.Count = saved.Latest, 0
	}
	if saved.Count >= notifyLimit {
		write(path, saved)
		return ""
	}
	saved.Count++
	write(path, saved)

	return "solitary " + saved.Latest + " is out (this is " + current + "). Update with: solitary update"
}

func recordFile() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update.json"), nil
}

// read returns a zero record for anything it cannot make sense of, which puts
// the next check in the past and refreshes the file.
func read(path string) record {
	data, err := os.ReadFile(path)
	if err != nil {
		return record{}
	}

	var saved record
	if err := json.Unmarshal(data, &saved); err != nil {
		return record{}
	}

	return saved
}

func write(path string, saved record) {
	data, err := json.Marshal(saved)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
