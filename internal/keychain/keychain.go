package keychain

import (
	"errors"
	"time"
)

var ErrLocked = errors.New("keychain is locked")
var ErrNotFound = errors.New("entry not found")

type Keychain interface {
	Exists() (bool, error)
	Create(password string, idleTimeout time.Duration) error
	Delete() error
	Get(service string) (string, error)
	Set(service, value string) error
	Remove(service string) error
	ListServices() ([]string, error)
	Unlock() error
	SetIdleTimeout(seconds int) error
	GetIdleTimeout() (int, error)
	IsLocked() (bool, error)
	AddToList() error
	Path() string
}
