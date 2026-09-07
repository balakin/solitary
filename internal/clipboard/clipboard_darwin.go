package clipboard

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// image reads the clipboard through osascript, which macOS ships — so a paste
// works on a fresh machine rather than after installing something.
//
// «class PNGf» is the clipboard's PNG flavour, and AppleScript hands raw data
// back in its own literal notation: «data PNGf89504e47...», hex, on one line.
func image() ([]byte, error) {
	out, err := run("osascript", "-e", "the clipboard as «class PNGf»")
	if err != nil {
		return nil, err
	}

	const prefix = "«data PNGf"
	literal := strings.TrimSpace(string(out))
	if !strings.HasPrefix(literal, prefix) || !strings.HasSuffix(literal, "»") {
		return nil, ErrNoImage
	}

	data, err := hex.DecodeString(strings.TrimSuffix(strings.TrimPrefix(literal, prefix), "»"))
	if err != nil {
		return nil, fmt.Errorf("decoding the clipboard image: %w", err)
	}

	return data, nil
}
