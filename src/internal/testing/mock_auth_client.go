package testing

import "go.uber.org/atomic"

type MockAuthClient struct {
	Calls atomic.Int32
}

func (m *MockAuthClient) GetAuthHeader() (string, error) {
	m.Calls.Inc()
	return "bearer stuff", nil
}
