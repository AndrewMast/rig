// Package keygen is the SSH IO seam: minting per-repo deploy keys. Private keys
// are born here and never leave the machine. Which key a repo uses is wired into
// that repo's local git config (core.sshCommand), not a global ~/.ssh/config, so
// nothing here touches shared SSH state. The real implementation shells out to
// ssh-keygen; a fake records calls for tests.
package keygen

// Request describes one deploy key's on-disk artifacts. The filename derives
// from the key ID upstream (model.Key), so distinct keys never collide.
type Request struct {
	SSHDir  string // directory holding keys (e.g. ~/.ssh)
	KeyFile string // private-key base filename, no directory
	Comment string // key comment / GitHub title hint
}

// Material is the result of generating (or reading) a key.
type Material struct {
	PrivPath  string
	PubPath   string
	PublicKey string // trimmed contents of the .pub file
}

// Keygen mints and removes deploy keys.
type Keygen interface {
	// Generate creates an ed25519 keypair.
	Generate(Request) (Material, error)
	// Remove deletes the keypair. Missing files are not an error.
	Remove(Request) error
	// PublicKey reads an existing public key.
	PublicKey(Request) (string, error)
}
