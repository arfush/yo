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

type fakeDialog struct{}

func (f *fakeDialog) Layout(layout.Context) {}

func (f *fakeDialog) LayoutFooter(layout.Context) {}

type fakeInstaller struct {
	screen shell.Screen
	dialog shell.Dialog
	title  string
}

func (f *fakeInstaller) SetScreen(content shell.Screen) { f.screen = content }

func (f *fakeInstaller) SetTitle(title string) { f.title = title }

func (f *fakeInstaller) SetDialog(dialog shell.Dialog) { f.dialog = dialog }

func testNavigator(t *testing.T) (*navigator.Navigator, *fakeScreen, *fakeDialog, *fakeInstaller) {
	t.Helper()
	startup := &fakeScreen{ScreenBase: navigator.ScreenBase{ScreenTitle: "start"}}
	dialog := &fakeDialog{}
	installer := &fakeInstaller{}
	nav := navigator.New(navigator.Options{
		Installer:  installer,
		Invalidate: func() {},
		Startup:    func() navigator.Screen { return startup },
	})
	return nav, startup, dialog, installer
}

func TestNewInstallsStartup(t *testing.T) {
	nav, startup, _, installer := testNavigator(t)

	if installer.screen != startup {
		t.Fatal("New did not install the startup screen")
	}
	if installer.title != "start" {
		t.Fatalf("title = %q; want start", installer.title)
	}
	if installer.dialog != nil {
		t.Fatal("New installed a dialog")
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
	nav, startup, _, installer := testNavigator(t)
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

func TestDependencyInstallationOpensOverStartupAndBackCloses(t *testing.T) {
	startup := &fakeScreen{ScreenBase: navigator.ScreenBase{ScreenTitle: "start"}}
	dialog := &fakeDialog{}
	installer := &fakeInstaller{}
	nav := navigator.New(navigator.Options{
		Installer:              installer,
		Invalidate:             func() {},
		Startup:                func() navigator.Screen { return startup },
		DependencyInstallation: func() shell.Dialog { return dialog },
	})
	nav.Apply()

	nav.Navigate(navigator.DependencyInstallation{})
	nav.Apply()
	if installer.dialog != dialog {
		t.Fatal("DependencyInstallation did not install the dialog")
	}
	if installer.screen != startup {
		t.Fatal("DependencyInstallation replaced the screen under the dialog")
	}

	nav.Navigate(navigator.Back{})
	nav.Apply()
	if installer.dialog != nil {
		t.Fatal("Back did not close the dialog")
	}
	if installer.screen != startup || startup.hidden != 0 || startup.closed != 0 {
		t.Fatal("Back closed the screen under the dialog")
	}
}

func TestCloseClosesScreens(t *testing.T) {
	nav, startup, _, installer := testNavigator(t)
	nav.Apply()

	nav.Close()
	nav.Close()
	if installer.dialog != nil {
		t.Fatal("Close left the dialog installed")
	}
	if startup.hidden != 1 || startup.closed != 1 {
		t.Fatalf("startup.OnHidden = %d, startup.OnClose = %d; want 1, 1", startup.hidden, startup.closed)
	}
}

func TestUnknownIntentPanics(t *testing.T) {
	nav, _, _, _ := testNavigator(t)
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
