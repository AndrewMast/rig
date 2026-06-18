// Package selfupdate holds the pure, testable pieces of `rig self update`:
// asset naming, checksum parsing/verification, and version comparison. The
// network download, attestation ladder, and atomic swap live in the CLI.
package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// AssetName is the release archive name for a platform.
func AssetName(version, goos, goarch string) string {
	return fmt.Sprintf("rig_%s_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), goos, goarch)
}

// ParseChecksums parses a GoReleaser checksums.txt ("<sha>  <name>" per line).
func ParseChecksums(data string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			out[fields[1]] = fields[0]
		}
	}
	return out
}

// FileSHA256 returns the lowercase hex SHA256 of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum confirms a file matches its listed checksum.
func VerifyChecksum(path, name string, checksums map[string]string) error {
	want, ok := checksums[name]
	if !ok {
		return fmt.Errorf("no checksum listed for %s", name)
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(want, got) {
		return fmt.Errorf("checksum mismatch for %s (want %s, got %s)", name, want, got)
	}
	return nil
}

// NeedsUpdate reports whether latest is newer than current. A "dev" build always
// updates; otherwise versions are compared numerically (semver), falling back to
// a string inequality for non-numeric components.
func NeedsUpdate(current, latest string) bool {
	if current == "dev" || current == "" {
		return true
	}
	c := splitVer(current)
	l := splitVer(latest)
	for i := 0; i < len(c) && i < len(l); i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return len(l) > len(c)
}

func splitVer(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// Drop any pre-release/build suffix.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}
