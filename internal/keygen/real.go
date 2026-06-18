package keygen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Exec is the real Keygen: it shells out to ssh-keygen and edits ~/.ssh/config.
type Exec struct{}

// New returns the real keygen.
func New() Exec { return Exec{} }

func (e Exec) privPath(r Request) string { return filepath.Join(r.SSHDir, r.KeyFile) }

// Generate mints an ed25519 keypair (no passphrase) and upserts the Host alias
// block. The SSH dir is created 0700 if missing.
func (e Exec) Generate(r Request) (Material, error) {
	if err := os.MkdirAll(r.SSHDir, 0o700); err != nil {
		return Material{}, fmt.Errorf("create ssh dir: %w", err)
	}
	priv := e.privPath(r)
	// ssh-keygen refuses to overwrite; clear any stale artifacts first.
	_ = os.Remove(priv)
	_ = os.Remove(priv + ".pub")

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", priv, "-N", "", "-C", r.Comment, "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		return Material{}, fmt.Errorf("ssh-keygen: %w: %s", err, strings.TrimSpace(string(out)))
	}
	pub, err := os.ReadFile(priv + ".pub")
	if err != nil {
		return Material{}, fmt.Errorf("read public key: %w", err)
	}
	if err := e.writeHostAlias(r, priv); err != nil {
		return Material{}, err
	}
	return Material{
		PrivPath:  priv,
		PubPath:   priv + ".pub",
		PublicKey: strings.TrimSpace(string(pub)),
	}, nil
}

// Remove deletes the keypair and its Host alias block.
func (e Exec) Remove(r Request) error {
	priv := e.privPath(r)
	_ = os.Remove(priv)
	_ = os.Remove(priv + ".pub")
	return e.editConfig(r, func(content string) string {
		return removeBlock(content, r.HostAlias)
	})
}

// PublicKey reads an existing public key file.
func (e Exec) PublicKey(r Request) (string, error) {
	pub, err := os.ReadFile(e.privPath(r) + ".pub")
	if err != nil {
		return "", fmt.Errorf("read public key: %w", err)
	}
	return strings.TrimSpace(string(pub)), nil
}

func (e Exec) writeHostAlias(r Request, identityFile string) error {
	return e.editConfig(r, func(content string) string {
		return upsertBlock(content, r.HostAlias, identityFile)
	})
}

// editConfig reads ~/.ssh/config, applies fn, and writes it back 0600.
func (e Exec) editConfig(r Request, fn func(string) string) error {
	cfgPath := filepath.Join(r.SSHDir, "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read ssh config: %w", err)
	}
	out := fn(string(data))
	if err := os.MkdirAll(r.SSHDir, 0o700); err != nil {
		return fmt.Errorf("create ssh dir: %w", err)
	}
	if err := os.WriteFile(cfgPath, []byte(out), 0o600); err != nil {
		return fmt.Errorf("write ssh config: %w", err)
	}
	return nil
}
