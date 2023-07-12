package mockprovider

import (
	"context"
	"time"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/dexidp/dex/connector/mock"
	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

const (
	ProviderName          = "mock"
	ProviderConnectorType = "mockCallback"
)

type MockProvider struct {
	Name        string
	Description string
	Type        string
	Owner       string
}

func New(p provider.ProviderCredential) (*MockProvider, error) {
	return &MockProvider{
		Name:  key.GetProviderName(p.Owner, ProviderName),
		Description: p.GetConnectorDescription(),
		Type:  ProviderConnectorType,
		Owner: p.Owner,
	}, nil
}

func (m *MockProvider) GetName() string {
	return m.Name
}

func (m *MockProvider) GetProviderName() string {
	return ProviderName
}

func (m *MockProvider) GetType() string {
	return m.Type
}

func (m *MockProvider) GetOwner() string {
	return m.Owner
}

func (m *MockProvider) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderApp, error) {
	connectorConfig := &mock.PasswordConfig{
		Username: "test",
		Password: "test",
	}
	data, err := yaml.Marshal(connectorConfig)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}
	return provider.ProviderApp{
		Connector: dex.Connector{
			Type:   m.Type,
			ID:     m.Name,
			Name:   m.Description,
			Config: string(data[:]),
		},
		SecretEndDateTime: time.Now().AddDate(0, 6, 0),
	}, nil
}

func (m *MockProvider) DeleteApp(name string, ctx context.Context) error {
	return nil
}

func (m *MockProvider) GetCredentialsForAuthenticatedApp(config provider.AppConfig) (map[string]string, error) {
	return map[string]string{
		"client-id":     "abc",
		"cert":          MockCert(),
		"client-secret": "test",
	}, nil
}
func (m *MockProvider) CleanCredentialsForAuthenticatedApp(config provider.AppConfig) error {
	return nil
}

func (m *MockProvider) DeleteAuthenticatedApp(config provider.AppConfig) error {
	return nil
}

func MockCert() string {
	return `-----BEGIN MOCK CERT-----
mock
cert
hello
-----END MOCK CERT-----`
}
