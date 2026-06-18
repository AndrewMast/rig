package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssetName(t *testing.T) {
	if got := AssetName("v1.2.3", "darwin", "arm64"); got != "rig_1.2.3_darwin_arm64.tar.gz" {
		t.Errorf("AssetName = %q", got)
	}
}

func TestParseAndVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rig_1.0.0_darwin_arm64.tar.gz")
	os.WriteFile(path, []byte("payload"), 0o644)
	sum, _ := FileSHA256(path)

	checks := ParseChecksums(sum + "  rig_1.0.0_darwin_arm64.tar.gz\notherhash  other.tar.gz\n")
	if checks["rig_1.0.0_darwin_arm64.tar.gz"] != sum {
		t.Fatal("checksum not parsed")
	}
	if err := VerifyChecksum(path, "rig_1.0.0_darwin_arm64.tar.gz", checks); err != nil {
		t.Errorf("verify: %v", err)
	}
	if err := VerifyChecksum(path, "missing.tar.gz", checks); err == nil {
		t.Error("expected error for missing checksum")
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.tar.gz")
	os.WriteFile(path, []byte("payload"), 0o644)
	checks := map[string]string{"f.tar.gz": "deadbeef"}
	if err := VerifyChecksum(path, "f.tar.gz", checks); err == nil {
		t.Error("expected checksum mismatch")
	}
}

func TestNeedsUpdate(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"dev", "v0.1.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.1.0", false},
		{"v1.0.0", "v1.0.1", true},
		{"v1.9.0", "v1.10.0", true},
	}
	for _, c := range cases {
		if got := NeedsUpdate(c.cur, c.latest); got != c.want {
			t.Errorf("NeedsUpdate(%q,%q) = %v, want %v", c.cur, c.latest, got, c.want)
		}
	}
}
