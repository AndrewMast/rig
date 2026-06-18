package cli

import "testing"

func TestPathWithin(t *testing.T) {
	cases := []struct {
		name      string
		dir, base string
		want      bool
	}{
		{"exact", "/Volumes/Development/Acme/widget", "/Volumes/Development/Acme/widget", true},
		{"nested", "/Volumes/Development/Acme/widget/src", "/Volumes/Development/Acme/widget", true},
		{"case-insensitive group", "/Volumes/Development/UrbanFavs/urbanfavs", "/Volumes/Development/Urbanfavs/urbanfavs", true},
		{"case-insensitive nested", "/Volumes/Development/ACME/widget/src", "/Volumes/Development/Acme/widget", true},
		{"sibling", "/Volumes/Development/Acme/widget2", "/Volumes/Development/Acme/widget", false},
		{"outside", "/Volumes/Development/Other/thing", "/Volumes/Development/Acme/widget", false},
		{"prefix-not-boundary", "/Volumes/Development/Acme/widgetry", "/Volumes/Development/Acme/widget", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pathWithin(c.dir, c.base); got != c.want {
				t.Errorf("pathWithin(%q, %q) = %v, want %v", c.dir, c.base, got, c.want)
			}
		})
	}
}
