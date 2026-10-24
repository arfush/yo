//go:build !windows

package ui

import (
	"errors"
	"image"
)

// tray is the notification-area icon. Only Windows implements the notification
// area; elsewhere the window controls keep their usual behaviour.
type tray struct{}

func traySupported() bool { return false }

func newTray(image.Image, func(), func()) (*tray, error) { return nil, nil }

func (t *tray) close() {}

func (t *tray) notifyMessage(string, string) error {
	return errors.New("уведомления Windows недоступны")
}

// ActivateRunningInstance has nothing to talk to outside Windows: the instance
// guard there is a lock file, and a second launch simply exits.
func ActivateRunningInstance() bool { return false }
