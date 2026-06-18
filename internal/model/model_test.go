package model

import "testing"

func TestProjectPathIsDerived(t *testing.T) {
	g := Group{Name: "Acme", Base: "/base"}
	p := Project{Group: "Acme", Name: "web-app"}

	if got, want := g.Path(), "/base/Acme"; got != want {
		t.Errorf("group path = %q, want %q", got, want)
	}
	if got, want := p.Path(g), "/base/Acme/web-app"; got != want {
		t.Errorf("project path = %q, want %q", got, want)
	}
	if got, want := p.ID(), "Acme/web-app"; got != want {
		t.Errorf("project id = %q, want %q", got, want)
	}
}

func TestKeyArtifactsDeriveFromID(t *testing.T) {
	k := Key{ID: "a1b2c3", Repo: "AndrewMast/gadget", Slug: "andrewmast-gadget", Write: true}
	if got, want := k.KeyFile(), "project_andrewmast-gadget_a1b2c3_deploy"; got != want {
		t.Errorf("key file = %q, want %q", got, want)
	}
	if got, want := k.Access(), "write"; got != want {
		t.Errorf("access = %q, want %q", got, want)
	}
}

func TestSlugForRepo(t *testing.T) {
	cases := map[string]string{
		"AndrewMast/gadget": "andrewmast-gadget",
		"Foo/Bar_Baz":       "foo-bar-baz",
		"a/b.c--d":          "a-b-c-d",
		"OWNER/repo.git":    "owner-repo-git",
	}
	for in, want := range cases {
		if got := SlugForRepo(in); got != want {
			t.Errorf("SlugForRepo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRepo(t *testing.T) {
	if o, r, ok := ParseRepo("  AndrewMast/gadget.git "); !ok || o != "AndrewMast" || r != "gadget" {
		t.Errorf("ParseRepo = %q/%q ok=%v", o, r, ok)
	}
	for _, bad := range []string{"", "noslash", "a/", "/b", "a/b/c"} {
		if _, _, ok := ParseRepo(bad); ok {
			t.Errorf("ParseRepo(%q) unexpectedly ok", bad)
		}
	}
}

func TestHosted(t *testing.T) {
	if (Project{Strategy: StrategyLocal}).Hosted() {
		t.Error("local project should not be hosted")
	}
	if !(Project{Strategy: StrategyDeployKey, Repo: "a/b"}).Hosted() {
		t.Error("deploy-key project with repo should be hosted")
	}
}
