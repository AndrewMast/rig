// Package keygen is the SSH IO seam: minting per-repo deploy keys and wiring the
// matching ~/.ssh/config Host alias. Private keys are born here and never leave
// the machine. The real implementation shells out to ssh-keygen; a fake records
// calls for tests.
package keygen

import (
	"fmt"
	"strings"
)

// Request describes one deploy key's on-disk artifacts. The filenames derive
// from the key ID upstream (model.Key), so distinct keys never collide.
type Request struct {
	SSHDir    string // directory holding keys and config (e.g. ~/.ssh)
	KeyFile   string // private-key base filename, no directory
	HostAlias string // ssh config Host alias (e.g. github.com-slug-id)
	Comment   string // key comment / GitHub title hint
}

// Material is the result of generating (or reading) a key.
type Material struct {
	PrivPath  string
	PubPath   string
	PublicKey string // trimmed contents of the .pub file
}

// Keygen mints and removes deploy keys and their Host alias blocks.
type Keygen interface {
	// Generate creates an ed25519 keypair and writes the Host alias block.
	Generate(Request) (Material, error)
	// Remove deletes the keypair and its Host alias block. Missing artifacts
	// are not an error.
	Remove(Request) error
	// PublicKey reads an existing public key.
	PublicKey(Request) (string, error)
}

const (
	blockBegin = "# >>> rig-managed: "
	blockEnd   = "# <<< rig-managed: "
)

// hostBlock renders the ssh config block for a host alias, fenced by markers so
// it can be removed cleanly later.
func hostBlock(hostAlias, identityFile string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s\n", blockBegin, hostAlias)
	fmt.Fprintf(&b, "Host %s\n", hostAlias)
	b.WriteString("\tHostName github.com\n")
	b.WriteString("\tUser git\n")
	fmt.Fprintf(&b, "\tIdentityFile %s\n", identityFile)
	b.WriteString("\tIdentitiesOnly yes\n")
	fmt.Fprintf(&b, "%s%s\n", blockEnd, hostAlias)
	return b.String()
}

// upsertBlock returns config content with the block for hostAlias replaced (or
// appended if absent). Pure, so it is unit-tested directly.
func upsertBlock(content, hostAlias, identityFile string) string {
	content = removeBlock(content, hostAlias)
	block := hostBlock(hostAlias, identityFile)
	if content == "" {
		return block
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + block
}

// removeBlock returns config content with the fenced block for hostAlias
// removed. If absent, content is returned unchanged.
func removeBlock(content, hostAlias string) string {
	begin := blockBegin + hostAlias
	end := blockEnd + hostAlias
	lines := strings.Split(content, "\n")
	var out []string
	skipping := false
	for _, ln := range lines {
		switch {
		case strings.TrimRight(ln, " ") == begin:
			skipping = true
		case strings.TrimRight(ln, " ") == end:
			skipping = false
		case !skipping:
			out = append(out, ln)
		}
	}
	// Trim trailing blank lines introduced by removal.
	res := strings.Join(out, "\n")
	res = strings.TrimRight(res, "\n")
	if res != "" {
		res += "\n"
	}
	return res
}
