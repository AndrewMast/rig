package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/AndrewMast/rig/internal/clock"
	"github.com/AndrewMast/rig/internal/config"
	"github.com/AndrewMast/rig/internal/gh"
	"github.com/AndrewMast/rig/internal/git"
	"github.com/AndrewMast/rig/internal/keygen"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/AndrewMast/rig/internal/ui"
)

// testApp builds an App backed by fakes and a temp RIG_HOME, with scripted stdin
// driving any prompts.
type testApp struct {
	*App
	gitFake *git.Fake
	ghFake  *gh.Fake
	keyFake *keygen.Fake
	out     *bytes.Buffer
}

func newTestApp(t *testing.T, stdin string) *testApp {
	t.Helper()
	home := t.TempDir()
	t.Setenv("RIG_HOME", home)
	paths, err := config.ResolvePaths()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	cfg := config.Default(paths)
	cfg.Handoff.Method = config.MethodPrint // no real clipboard/gh side effects in tests

	out := &bytes.Buffer{}
	gf := git.NewFake()
	ghf := gh.NewFake()
	kf := keygen.NewFake()
	app := &App{
		Paths:  paths,
		Config: cfg,
		Store:  registry.NewStore(paths.Registry),
		Git:    gf,
		GH:     ghf,
		Keygen: kf,
		Clock:  clock.Fixed{},
		UI:     ui.NewWith(strings.NewReader(stdin), out),
	}
	return &testApp{App: app, gitFake: gf, ghFake: ghf, keyFake: kf, out: out}
}

// run executes a command line against the test app and returns combined command
// output.
func (ta *testApp) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd(ta.App)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	ctx := withApp(context.Background(), ta.App)
	err := root.ExecuteContext(ctx)
	return buf.String(), err
}

func (ta *testApp) reg(t *testing.T) *registry.Registry {
	t.Helper()
	r, err := ta.Registry()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return r
}
