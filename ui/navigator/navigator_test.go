package navigator

import (
	"testing"

	"gioui.org/layout"

	"github.com/arfush/yo/ui/shell"
)

type fakeScreen struct {
	ScreenBase
	shown  int
	hidden int
	closed int
}

func (f *fakeScreen) Layout(layout.Context) {}

func (f *fakeScreen) OnShown() { f.shown++ }

func (f *fakeScreen) OnHidden() { f.hidden++ }

func (f *fakeScreen) OnClose() { f.closed++ }

type fakeInstaller struct {
	screen shell.Screen
	title  string
}

func (f *fakeInstaller) SetScreen(content shell.Screen) { f.screen = content }

func (f *fakeInstaller) SetTitle(title string) { f.title = title }

func testNavigator(t *testing.T) (*Navigator, *fakeScreen, *fakeInstaller) {
	t.Helper()
	startup := &fakeScreen{ScreenBase: ScreenBase{ScreenTitle: "start"}}
	installer := &fakeInstaller{}
	nav := New(Options{
		Installer:  installer,
		Invalidate: func() {},
		Startup:    func(Handler) Screen { return startup },
	})
	return nav, startup, installer
}

func TestNewInstallsStartup(t *testing.T) {
	nav, startup, installer := testNavigator(t)

	if installer.screen != startup {
		t.Fatal("New did not install the startup screen")
	}
	if installer.title != "start" {
		t.Fatalf("title = %q; want start", installer.title)
	}
	if startup.shown != 0 {
		t.Fatalf("startup.OnShown = %d before Apply; want 0", startup.shown)
	}

	nav.Apply()
	if startup.shown != 1 {
		t.Fatalf("startup.OnShown = %d after Apply; want 1", startup.shown)
	}
}

func TestNavigatePushesScreen(t *testing.T) {
	child := &fakeScreen{ScreenBase: ScreenBase{ScreenTitle: "child"}}
	startup := &fakeScreen{ScreenBase: ScreenBase{ScreenTitle: "start"}}
	installer := &fakeInstaller{}
	invalidated := 0
	nav := New(Options{
		Installer:  installer,
		Invalidate: func() { invalidated++ },
		Startup:    func(Handler) Screen { return startup },
		Routes: map[Route]ScreenFactory{
			"child": func(Handler) Screen { return child },
		},
	})
	nav.Apply()

	nav.Navigate(Intent{Route: "child"})
	if invalidated != 1 {
		t.Fatalf("invalidate calls = %d; want 1", invalidated)
	}
	if installer.screen != startup {
		t.Fatal("Navigate installed the screen before Apply")
	}

	nav.Apply()
	if installer.screen != child {
		t.Fatal("Apply did not install the pushed screen")
	}
	if installer.title != "child" {
		t.Fatalf("title = %q; want child", installer.title)
	}
	if startup.hidden != 1 || child.shown != 1 {
		t.Fatalf("startup.OnHidden = %d, child.OnShown = %d; want 1, 1", startup.hidden, child.shown)
	}
}

func TestUnknownRoutePanics(t *testing.T) {
	nav, _, _ := testNavigator(t)
	nav.Navigate(Intent{Route: "missing"})

	defer func() {
		if recover() == nil {
			t.Fatal("unknown route did not panic")
		}
	}()
	nav.Apply()
}

func TestCloseRunsLifecycle(t *testing.T) {
	nav, startup, _ := testNavigator(t)
	nav.Apply()

	nav.Close()
	nav.Close()
	if startup.hidden != 1 || startup.closed != 1 {
		t.Fatalf("startup.OnHidden = %d, startup.OnClose = %d; want 1, 1", startup.hidden, startup.closed)
	}
}

func TestNavigateAfterCloseIsIgnored(t *testing.T) {
	invalidated := 0
	startup := &fakeScreen{}
	nav := New(Options{
		Installer:  &fakeInstaller{},
		Invalidate: func() { invalidated++ },
		Startup:    func(Handler) Screen { return startup },
	})
	nav.Close()

	nav.Navigate(Intent{Route: "child"})
	if invalidated != 0 {
		t.Fatalf("invalidate calls after Close = %d; want 0", invalidated)
	}
}
