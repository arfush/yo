package ui

import "testing"

func TestFooterButtonHasNoBuiltInAppearance(t *testing.T) {
	var button footerNamedButton
	if button.label != "" {
		t.Fatalf("base button has a default label: %q", button.label)
	}
	if button.icon != footerButtonNoIcon {
		t.Fatalf("base button has a default icon: %v", button.icon)
	}
	if button.style != (footerButtonStyle{}) {
		t.Fatalf("base button has a default style: %+v", button.style)
	}
}

func TestBackAndWatchPresetsShareTheButtonShape(t *testing.T) {
	back := newBackFooterButton()
	watch := newWatchFooterButton()
	if back.label != "Вернуться" || back.icon != footerButtonBackIcon {
		t.Fatalf("unexpected back preset: %+v", back)
	}
	if watch.label != "Смотреть" || watch.icon != footerButtonPlayIcon {
		t.Fatalf("unexpected watch preset: %+v", watch)
	}
	if back.size != watch.size {
		t.Fatalf("back size %v != watch size %v", back.size, watch.size)
	}
}
