package model

import "testing"

func TestProjectPathIsDerived(t *testing.T) {
	g := Group{Name: "Dripstone", Base: "/Volumes/Development"}
	p := Project{Group: "Dripstone", Name: "flutter-app"}

	if got, want := g.Path(), "/Volumes/Development/Dripstone"; got != want {
		t.Errorf("group path = %q, want %q", got, want)
	}
	if got, want := p.Path(g), "/Volumes/Development/Dripstone/flutter-app"; got != want {
		t.Errorf("project path = %q, want %q", got, want)
	}
	if got, want := p.ID(), "Dripstone/flutter-app"; got != want {
		t.Errorf("project id = %q, want %q", got, want)
	}
}

func TestKeyArtifactsDeriveFromID(t *testing.T) {
	k := Key{ID: "a1b2c3", Repo: "AndrewMast/dripstone", Slug: "andrewmast-dripstone", Write: true}
	if got, want := k.KeyFile(), "project_andrewmast-dripstone_a1b2c3_deploy"; got != want {
		t.Errorf("key file = %q, want %q", got, want)
	}
	if got, want := k.Access(), "write"; got != want {
		t.Errorf("access = %q, want %q", got, want)
	}
}

func TestSlugForRepo(t *testing.T) {
	cases := map[string]string{
		"AndrewMast/dripstone": "andrewmast-dripstone",
		"Foo/Bar_Baz":          "foo-bar-baz",
		"a/b.c--d":             "a-b-c-d",
		"OWNER/repo.git":       "owner-repo-git",
	}
	for in, want := range cases {
		if got := SlugForRepo(in); got != want {
			t.Errorf("SlugForRepo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRepo(t *testing.T) {
	if o, r, ok := ParseRepo("  AndrewMast/dripstone.git "); !ok || o != "AndrewMast" || r != "dripstone" {
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
