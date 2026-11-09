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

func testNavigator(t *testing.T) (*Navigator, *fakeScreen, *fakeDialog, *fakeInstaller) {
	t.Helper()
	startup := &fakeScreen{ScreenBase: ScreenBase{ScreenTitle: "start"}}
	dialog := &fakeDialog{}
	installer := &fakeInstaller{}
	nav := New(Options{
		Installer:  installer,
		Invalidate: func() {},
		Startup:    func() Screen { return startup },
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

func TestBackReturnsToPreviousScreen(t *testing.T) {
	nav, startup, _, installer := testNavigator(t)
	child := &fakeScreen{ScreenBase: ScreenBase{ScreenTitle: "child"}}
	nav.Apply()

	nav.push(child)
	if installer.screen != child {
		t.Fatal("push did not install the child screen")
	}
	if startup.hidden != 1 || child.shown != 1 {
		t.Fatalf("startup.OnHidden = %d, child.OnShown = %d; want 1, 1", startup.hidden, child.shown)
	}

	nav.Navigate(Back{})
	nav.Apply()
	if installer.screen != startup {
		t.Fatal("Back did not install the previous screen")
	}
	if installer.title != "start" {
		t.Fatalf("title = %q; want start", installer.title)
	}
	if child.hidden != 1 || child.closed != 1 {
		t.Fatalf("child.OnHidden = %d, child.OnClose = %d; want 1, 1", child.hidden, child.closed)
	}
	if startup.shown != 2 {
		t.Fatalf("startup.OnShown = %d; want 2", startup.shown)
	}
}

func TestBackAtRootDoesNothing(t *testing.T) {
	nav, startup, _, installer := testNavigator(t)
	nav.Apply()

	nav.Navigate(Back{})
	nav.Apply()
	if installer.screen != startup {
		t.Fatal("Back replaced the root screen")
	}
	if startup.hidden != 0 || startup.closed != 0 {
		t.Fatalf("startup.OnHidden = %d, startup.OnClose = %d; want 0, 0", startup.hidden, startup.closed)
	}
}

func TestDialogOpens(t *testing.T) {
	nav, startup, dialog, installer := testNavigator(t)
	nav.Apply()

	nav.openDialog(dialog)
	if installer.dialog != dialog {
		t.Fatal("openDialog did not install the dialog")
	}
	if startup.hidden != 0 {
		t.Fatalf("startup.OnHidden = %d while the dialog is open; want 0", startup.hidden)
	}
}

func TestBackClosesDialogBeforeScreen(t *testing.T) {
	nav, startup, _, installer := testNavigator(t)
	child := &fakeScreen{ScreenBase: ScreenBase{ScreenTitle: "child"}}
	nav.Apply()
	nav.push(child)
	nav.openDialog(&fakeDialog{})

	nav.Navigate(Back{})
	nav.Apply()
	if installer.dialog != nil {
		t.Fatal("Back did not close the dialog")
	}
	if installer.screen != child {
		t.Fatal("Back closed the screen together with the dialog")
	}
	if child.hidden != 0 {
		t.Fatalf("child.OnHidden = %d after closing the dialog; want 0", child.hidden)
	}

	nav.Navigate(Back{})
	nav.Apply()
	if installer.screen != startup {
		t.Fatal("second Back did not return to the previous screen")
	}
	if child.hidden != 1 || child.closed != 1 {
		t.Fatalf("child.OnHidden = %d, child.OnClose = %d; want 1, 1", child.hidden, child.closed)
	}
}

func TestCloseClearsDialogAndScreens(t *testing.T) {
	nav, startup, _, installer := testNavigator(t)
	nav.Apply()
	nav.openDialog(&fakeDialog{})

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
	nav := New(Options{
		Installer:  &fakeInstaller{},
		Invalidate: func() { invalidated++ },
		Startup:    func() Screen { return startup },
	})
	nav.Close()

	nav.Navigate(Back{})
	if invalidated != 0 {
		t.Fatalf("invalidate calls after Close = %d; want 0", invalidated)
	}
}
