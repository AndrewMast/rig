package keygen

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRealGenerateAndRemove(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	dir := t.TempDir()
	r := Request{
		SSHDir:  dir,
		KeyFile: "project_test_abc123_deploy",
		Comment: "rig test",
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

	pub, err := e.PublicKey(r)
	if err != nil || pub != mat.PublicKey {
		t.Errorf("PublicKey = %q, %v", pub, err)
	}

	if err := e.Remove(r); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(mat.PrivPath); !os.IsNotExist(err) {
		t.Error("private key not removed")
	}
}

func TestFakeRecordsKeyByFile(t *testing.T) {
	f := NewFake()
	r := Request{SSHDir: "/ssh", KeyFile: "project_a_1_deploy", Comment: "c"}
	mat, err := f.Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	if mat.PrivPath != "/ssh/project_a_1_deploy" {
		t.Errorf("priv path = %q", mat.PrivPath)
	}
	if pub, _ := f.PublicKey(r); pub != mat.PublicKey {
		t.Error("public key mismatch")
	}
	if err := f.Remove(r); err != nil {
		t.Fatal(err)
	}
	if _, err := f.PublicKey(r); err == nil {
		t.Error("expected key gone after remove")
	}
	if len(f.Generated) != 1 || len(f.Removed) != 1 {
		t.Errorf("records: gen=%d rm=%d", len(f.Generated), len(f.Removed))
	}
}
