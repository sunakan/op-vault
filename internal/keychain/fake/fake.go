package fake

import (
	"time"

	"github.com/sunakan/op-keychain/internal/keychain"
)

type Keychain struct {
	Entries     map[string]string
	Locked      bool
	ExistsVal   bool
	PathVal     string
	IdleTimeout int
	UnlockErr   error
	CreateErr   error
	DeleteErr   error
}

func New() *Keychain {
	return &Keychain{
		Entries:     make(map[string]string),
		ExistsVal:   true,
		PathVal:     "/fake/op-keychain.keychain-db",
		IdleTimeout: 3600,
	}
}

func (k *Keychain) Exists() (bool, error)  { return k.ExistsVal, nil }
func (k *Keychain) Path() string           { return k.PathVal }
func (k *Keychain) AddToList() error       { return nil }
func (k *Keychain) GetIdleTimeout() (int, error) { return k.IdleTimeout, nil }
func (k *Keychain) IsLocked() (bool, error)      { return k.Locked, nil }

func (k *Keychain) Create(password string, idleTimeout time.Duration) error {
	if k.CreateErr != nil {
		return k.CreateErr
	}
	k.ExistsVal = true
	k.IdleTimeout = int(idleTimeout.Seconds())
	return nil
}

func (k *Keychain) Delete() error {
	if k.DeleteErr != nil {
		return k.DeleteErr
	}
	k.ExistsVal = false
	k.Entries = make(map[string]string)
	return nil
}

func (k *Keychain) Unlock() error {
	if k.UnlockErr != nil {
		return k.UnlockErr
	}
	k.Locked = false
	return nil
}

func (k *Keychain) SetIdleTimeout(seconds int) error {
	k.IdleTimeout = seconds
	return nil
}

func (k *Keychain) Get(service string) (string, error) {
	if k.Locked {
		return "", keychain.ErrLocked
	}
	v, ok := k.Entries[service]
	if !ok {
		return "", keychain.ErrNotFound
	}
	return v, nil
}

func (k *Keychain) Set(service, value string) error {
	if k.Locked {
		return keychain.ErrLocked
	}
	k.Entries[service] = value
	return nil
}

func (k *Keychain) Remove(service string) error {
	if k.Locked {
		return keychain.ErrLocked
	}
	if _, ok := k.Entries[service]; !ok {
		return keychain.ErrNotFound
	}
	delete(k.Entries, service)
	return nil
}

func (k *Keychain) ListServices() ([]string, error) {
	services := make([]string, 0, len(k.Entries))
	for s := range k.Entries {
		services = append(services, s)
	}
	return services, nil
}
