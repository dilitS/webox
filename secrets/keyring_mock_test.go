package secrets

import "github.com/zalando/go-keyring"

type mockKeyringClient struct {
	setErr    error
	getErr    error
	deleteErr error
	getValue  string

	values       map[string]string
	deletedProbe bool
}

func (m *mockKeyringClient) Set(service, user, password string) error {
	if m.setErr != nil {
		return m.setErr
	}
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[service+"\x00"+user] = password
	return nil
}

func (m *mockKeyringClient) Get(service, user string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	if m.getValue != "" {
		return m.getValue, nil
	}
	if m.values == nil {
		return "", keyring.ErrNotFound
	}
	value, ok := m.values[service+"\x00"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (m *mockKeyringClient) Delete(service, user string) error {
	if service == keyringService && user == probeKey {
		m.deletedProbe = true
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.values == nil {
		return keyring.ErrNotFound
	}
	delete(m.values, service+"\x00"+user)
	return nil
}
