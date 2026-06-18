package gh

import "context"

// Fake is a canned GH for tests.
type Fake struct {
	HasToken   bool
	UserLogin  string
	ExistsRepo map[string]bool // "owner/repo" -> exists
	PublicRepo map[string]bool // "owner/repo" -> public
	VerifyErr  error
}

// NewFake returns an empty fake.
func NewFake() *Fake {
	return &Fake{ExistsRepo: map[string]bool{}, PublicRepo: map[string]bool{}}
}

func (f *Fake) Available() bool { return f.HasToken }

func (f *Fake) Verify(context.Context) error { return f.VerifyErr }

func (f *Fake) Login(context.Context) (string, error) { return f.UserLogin, nil }

func (f *Fake) RepoExists(_ context.Context, owner, repo string) (bool, error) {
	return f.ExistsRepo[owner+"/"+repo], nil
}

func (f *Fake) RepoPublic(_ context.Context, owner, repo string) (bool, error) {
	return f.PublicRepo[owner+"/"+repo], nil
}
