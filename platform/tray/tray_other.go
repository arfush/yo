//go:build !windows

package tray

import (
	"errors"
	"image"
)

// tray is the notification-area icon. Only Windows implements the notification
// area; elsewhere the window controls keep their usual behaviour.
type Tray struct{}

func Supported() bool { return false }

func New(image.Image, func(), func()) (*Tray, error) { return nil, nil }

func (t *Tray) Close() {}

func (t *Tray) NotifyMessage(string, string) error {
	return errors.New("уведомления Windows недоступны")
}

// ActivateRunningInstance has nothing to talk to outside Windows: the instance
// guard there is a lock file, and a second launch simply exits.
func ActivateRunningInstance() bool { return false }
