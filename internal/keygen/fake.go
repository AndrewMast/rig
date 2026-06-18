package keygen

import "fmt"

// Fake is an in-memory Keygen for tests. It records generated and removed
// requests and hands back deterministic public keys.
type Fake struct {
	Generated []Request
	Removed   []Request
	// Keys maps HostAlias to a canned public key; a default is returned when
	// no entry exists.
	Keys map[string]string
}

// NewFake returns an empty fake.
func NewFake() *Fake { return &Fake{Keys: map[string]string{}} }

// Generate records the request and returns a deterministic public key.
func (f *Fake) Generate(r Request) (Material, error) {
	f.Generated = append(f.Generated, r)
	pub := f.Keys[r.HostAlias]
	if pub == "" {
		pub = fmt.Sprintf("ssh-ed25519 AAAAFAKE-%s %s", r.HostAlias, r.Comment)
		f.Keys[r.HostAlias] = pub
	}
	priv := r.SSHDir + "/" + r.KeyFile
	return Material{PrivPath: priv, PubPath: priv + ".pub", PublicKey: pub}, nil
}

// Remove records the removal.
func (f *Fake) Remove(r Request) error {
	f.Removed = append(f.Removed, r)
	delete(f.Keys, r.HostAlias)
	return nil
}

// PublicKey returns the canned key for the request's host alias.
func (f *Fake) PublicKey(r Request) (string, error) {
	if pub, ok := f.Keys[r.HostAlias]; ok {
		return pub, nil
	}
	return "", fmt.Errorf("no key for %s", r.HostAlias)
}
