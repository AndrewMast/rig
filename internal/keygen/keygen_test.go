package keygen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertAndRemoveBlock(t *testing.T) {
	cfg := upsertBlock("", "github.com-a-1", "/ssh/key_a")
	if !strings.Contains(cfg, "Host github.com-a-1") || !strings.Contains(cfg, "IdentityFile /ssh/key_a") {
		t.Fatalf("block not written:\n%s", cfg)
	}

	// Upsert a second alias keeps the first.
	cfg = upsertBlock(cfg, "github.com-b-2", "/ssh/key_b")
	if !strings.Contains(cfg, "Host github.com-a-1") || !strings.Contains(cfg, "Host github.com-b-2") {
		t.Fatalf("second upsert dropped first:\n%s", cfg)
	}

	// Re-upserting the first replaces, not duplicates.
	cfg = upsertBlock(cfg, "github.com-a-1", "/ssh/key_a_new")
	if strings.Count(cfg, "Host github.com-a-1") != 1 {
		t.Fatalf("duplicate block:\n%s", cfg)
	}
	if !strings.Contains(cfg, "/ssh/key_a_new") {
		t.Fatalf("upsert did not replace identity file:\n%s", cfg)
	}

	// Remove leaves the other intact.
	cfg = removeBlock(cfg, "github.com-a-1")
	if strings.Contains(cfg, "github.com-a-1") {
		t.Fatalf("block not removed:\n%s", cfg)
	}
	if !strings.Contains(cfg, "Host github.com-b-2") {
		t.Fatalf("removal clobbered sibling:\n%s", cfg)
	}
}

func TestRemoveBlockPreservesUserContent(t *testing.T) {
	user := "Host example\n\tHostName example.com\n"
	cfg := upsertBlock(user, "github.com-a-1", "/ssh/key_a")
	cfg = removeBlock(cfg, "github.com-a-1")
	if !strings.Contains(cfg, "Host example") {
		t.Fatalf("user content lost:\n%q", cfg)
	}
}

func TestRealGenerateAndRemove(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dir := t.TempDir()
	r := Request{
		SSHDir:    dir,
		KeyFile:   "project_test_abc123_deploy",
		HostAlias: "github.com-test-abc123",
		Comment:   "rig test",
	}
	e := New()
	mat, err := e.Generate(r)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(mat.PublicKey, "ssh-ed25519 ") {
		t.Errorf("unexpected public key: %q", mat.PublicKey)
	}
	if _, err := os.Stat(mat.PrivPath); err != nil {
		t.Errorf("private key missing: %v", err)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, "config"))
	if !strings.Contains(string(cfg), r.HostAlias) {
		t.Error("host alias not in ssh config")
	}

	if err := e.Remove(r); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(mat.PrivPath); !os.IsNotExist(err) {
		t.Error("private key not removed")
	}
	cfg, _ = os.ReadFile(filepath.Join(dir, "config"))
	if strings.Contains(string(cfg), r.HostAlias) {
		t.Error("host alias not removed from ssh config")
	}
}
