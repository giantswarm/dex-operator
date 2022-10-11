package mockprovider

import (
	"giantswarm/dex-operator/pkg/idp/provider"

	"github.com/dexidp/dex/connector/mock"
)

const (
	ProviderName = "mock"
)

type MockProvider struct {
	Name string
}

func New(p provider.ProviderCredential) (*MockProvider, error) {
	return &MockProvider{
		Name: ProviderName,
	}, nil
}

func (m *MockProvider) CreateApp(config provider.AppConfig) (provider.Connector, error) {
	return provider.Connector{
		Type: "mockCallback",
		ID:   "mock",
		Name: "Example",
		Config: &mock.PasswordConfig{
			Username: "test",
			Password: "test",
		},
	}, nil
}

func (m *MockProvider) DeleteApp(name string) error {
	return nil
}
