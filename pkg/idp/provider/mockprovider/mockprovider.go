package mockprovider

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"

	"github.com/dexidp/dex/connector/mock"
)

const (
	ProviderName          = "mock"
	ProviderConnectorType = "mockCallback"
)

type MockProvider struct {
	Name  string
	Type  string
	Owner string
}

func New(p provider.ProviderCredential) (*MockProvider, error) {
	return &MockProvider{
		Name:  ProviderName,
		Type:  ProviderConnectorType,
		Owner: "giantswarm",
	}, nil
}

func (m *MockProvider) GetName() string {
	return m.Name
}

func (m *MockProvider) GetType() string {
	return m.Type
}

func (m *MockProvider) GetOwner() string {
	return m.Owner
}

func (m *MockProvider) CreateApp(config provider.AppConfig, ctx context.Context) (dex.Connector, error) {
	return dex.Connector{
		Type: m.Type,
		ID:   m.Name,
		Name: key.GetConnectorDescription(ProviderConnectorType, m.Owner),
		Config: &mock.PasswordConfig{
			Username: "test",
			Password: "test",
		},
	}, nil
}

func (m *MockProvider) DeleteApp(name string) error {
	return nil
}
