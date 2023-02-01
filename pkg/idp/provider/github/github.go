package github

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"

	githubconnector "github.com/dexidp/dex/connector/github"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
)

const (
	ProviderName          = "github"
	ProviderConnectorType = "github"
	OrganizationKey       = "organization"
	TeamKey               = "team"
	ClientIDKey           = "client-id"
	ClientSecretKey       = "client-secret"
)

type Github struct {
	Client       string
	Log          *logr.Logger
	Name         string
	Type         string
	Owner        string
	Organization string
	Team         string
}

func New(p provider.ProviderCredential, log *logr.Logger) (*Github, error) {
	var organization, team, clientID, clientSecret string
	{
		if log == nil {
			return nil, microerror.Maskf(invalidConfigError, "Logger must not be empty.")
		}
		if p.Name == "" {
			return nil, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
		}
		if p.Owner == "" {
			return nil, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
		}
		if organization = p.Credentials[OrganizationKey]; organization == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", OrganizationKey)
		}
		if team = p.Credentials[TeamKey]; team == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", TeamKey)
		}
		if clientID = p.Credentials[ClientIDKey]; clientID == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientIDKey)
		}
		if clientSecret = p.Credentials[ClientSecretKey]; clientSecret == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientSecretKey)
		}
	}
	var client string
	{
		client = ""
	}
	return &Github{
		Name:         key.GetProviderName(p.Owner, p.Name),
		Log:          log,
		Type:         ProviderConnectorType,
		Client:       client,
		Owner:        p.Owner,
		Organization: organization,
		Team:         team,
	}, nil
}

func (g *Github) GetName() string {
	return g.Name
}

func (g *Github) GetType() string {
	return g.Type
}

func (g *Github) GetOwner() string {
	return g.Owner
}

func (g *Github) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderApp, error) {
	// Create or update application registration
	id, err := g.createOrUpdateApplication(config, ctx)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	// Retrieve old secret
	oldSecret, err := getSecretFromConfig(oldConnector.Config)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	//Create or update secret
	secret, err := g.createOrUpdateSecret(id, config, ctx, oldSecret)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	connectorConfig := &githubconnector.Config{
		ClientID:     secret.ClientId,
		ClientSecret: secret.ClientSecret,
		Orgs: []githubconnector.Org{
			{
				Name:  g.Organization,
				Teams: []string{g.Team},
			},
		},
		RedirectURI: config.RedirectURI,
	}
	data, err := yaml.Marshal(connectorConfig)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}
	return provider.ProviderApp{
		Connector: dex.Connector{
			Type:   g.Type,
			ID:     g.Name,
			Name:   key.GetConnectorDescription(ProviderConnectorType, g.Owner),
			Config: string(data[:]),
		},
		SecretEndDateTime: secret.EndDateTime,
	}, nil
}

func (g *Github) DeleteApp(name string, ctx context.Context) error {
	return nil
}

func (g *Github) createOrUpdateApplication(config provider.AppConfig, ctx context.Context) (string, error) {
	return "", nil
}

func (g *Github) createOrUpdateSecret(id string, config provider.AppConfig, ctx context.Context, oldSecret string) (provider.ProviderSecret, error) {
	return provider.ProviderSecret{}, nil
}
