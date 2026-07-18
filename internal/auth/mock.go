package auth

import "context"

// MockTokenProvider is a mock implementation of a token provider for testing
type MockTokenProvider struct {
	Token string
	Err   error
}

func (m *MockTokenProvider) GetGoogleAccessToken(ctx context.Context, userID string) (string, error) {
	return m.Token, m.Err
}

// MockPlayrunAuthenticator is a mock implementation of PlayrunAuthenticator for
// testing endpoint handlers that depend on Playrun login.
type MockPlayrunAuthenticator struct {
	Token string
	Err   error
}

func (m *MockPlayrunAuthenticator) Login(email, password string) (string, error) {
	return m.Token, m.Err
}
