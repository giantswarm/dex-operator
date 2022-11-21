package mockprovider

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"

	"github.com/dexidp/dex/connector/mock"
	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
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
		Name:  key.GetProviderName(p.Owner, ProviderName),
		Type:  ProviderConnectorType,
		Owner: p.Owner,
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

func (m *MockProvider) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context) (dex.Connector, error) {
	connectorConfig := &mock.PasswordConfig{
		Username: "test",
		Password: "test",
	}
	data, err := yaml.Marshal(connectorConfig)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	return dex.Connector{
		Type:   m.Type,
		ID:     m.Name,
		Name:   key.GetConnectorDescription(ProviderConnectorType, m.Owner),
		Config: string(data[:]),
	}, nil
}

func (m *MockProvider) DeleteApp(name string, ctx context.Context) error {
	return nil
}
