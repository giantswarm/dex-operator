// Package connectors contains local copies of Dex connector configuration types.
//
// Source: https://github.com/dexidp/dex/blob/v2.42.0/connector/mock/mock.go
package connectors

// MockPasswordConfig holds configuration for mock password connector.
// This is used for testing only.
type MockPasswordConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
