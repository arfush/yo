package ui

import (
	"image"

	platformtray "github.com/arfush/yo/platform/tray"
)

// trayAdaptor keeps the screen code on its old method names while the Win32
// implementation lives in platform/tray. The wrapper also tolerates a nil
// handle, which the first-run path can produce when the notification area is
// unavailable.
type tray struct {
	*platformtray.Tray
}

func traySupported() bool { return platformtray.Supported() }

func newTray(icon image.Image, onOpen, onExit func()) (*tray, error) {
	value, err := platformtray.New(icon, onOpen, onExit)
	if value == nil {
		return nil, err
	}
	return &tray{Tray: value}, nil
}

func (t *tray) close() {
	if t != nil && t.Tray != nil {
		t.Tray.Close()
	}
}

func (t *tray) notifyMessage(title, body string) error {
	if t == nil || t.Tray == nil {
		return nil
	}
	return t.Tray.NotifyMessage(title, body)
}

// Notification limits and trimming live in the tray package because they come
// from NOTIFYICONDATAW. The screen code keeps short names for its formatting
// helpers and tests.
const (
	maxNotificationTitle = platformtray.MaxNotificationTitle
	maxNotificationBody  = platformtray.MaxNotificationBody
)

func trimNotificationText(value string, maxUnits int) string {
	return platformtray.TrimNotificationText(value, maxUnits)
}
