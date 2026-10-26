package assets

import "testing"

func TestIconsLoad(t *testing.T) {
	for _, name := range IconNames {
		if icon := Icon(name); icon == nil {
			t.Errorf("icon %q is missing or invalid", name)
		}
	}
}

func TestFontsRead(t *testing.T) {
	for _, name := range []string{
		"Inter-Regular.ttf",
		"Inter-Medium.ttf",
		"Inter-SemiBold.ttf",
		"Inter-Bold.ttf",
	} {
		data, err := ReadFont(name)
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if len(data) < 1000 {
			t.Errorf("%s is unexpectedly small: %d bytes", name, len(data))
		}
	}
}

func TestAppIconDecodes(t *testing.T) {
	icon, err := AppIcon()
	if err != nil {
		t.Fatalf("AppIcon: %v", err)
	}
	if icon.Bounds().Empty() {
		t.Fatal("app icon is empty")
	}
}
