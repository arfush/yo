package navigator_test

import (
	"testing"

	"gioui.org/layout"

	"github.com/arfush/yo/ui/navigator"
	"github.com/arfush/yo/ui/shell"
)

type fakeScreen struct {
	navigator.ScreenBase
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

func testNavigator(t *testing.T) (*navigator.Navigator, *fakeScreen, *fakeInstaller) {
	t.Helper()
	startup := &fakeScreen{ScreenBase: navigator.ScreenBase{ScreenTitle: "start"}}
	installer := &fakeInstaller{}
	nav := navigator.New(navigator.Options{
		Installer:  installer,
		Invalidate: func() {},
		Startup:    func() navigator.Screen { return startup },
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

func TestBackAtRootDoesNothing(t *testing.T) {
	nav, startup, installer := testNavigator(t)
	nav.Apply()

	nav.Navigate(navigator.Back{})
	nav.Apply()
	if installer.screen != startup {
		t.Fatal("Back replaced the root screen")
	}
	if startup.hidden != 0 || startup.closed != 0 {
		t.Fatalf("startup.OnHidden = %d, startup.OnClose = %d; want 0, 0", startup.hidden, startup.closed)
	}
}

func TestCloseClosesScreens(t *testing.T) {
	nav, startup, _ := testNavigator(t)
	nav.Apply()

	nav.Close()
	nav.Close()
	if startup.hidden != 1 || startup.closed != 1 {
		t.Fatalf("startup.OnHidden = %d, startup.OnClose = %d; want 1, 1", startup.hidden, startup.closed)
	}
}

func TestUnknownIntentPanics(t *testing.T) {
	nav, _, _ := testNavigator(t)
	nav.Navigate(struct{}{})

	defer func() {
		if recover() == nil {
			t.Fatal("unknown intent did not panic")
		}
	}()
	nav.Apply()
}

func TestNavigateAfterCloseIsIgnored(t *testing.T) {
	invalidated := 0
	startup := &fakeScreen{}
	nav := navigator.New(navigator.Options{
		Installer:  &fakeInstaller{},
		Invalidate: func() { invalidated++ },
		Startup:    func() navigator.Screen { return startup },
	})
	nav.Close()

	nav.Navigate(navigator.Back{})
	if invalidated != 0 {
		t.Fatalf("invalidate calls after Close = %d; want 0", invalidated)
	}
}
