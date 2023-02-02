package github

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"time"

	githubconnector "github.com/dexidp/dex/connector/github"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	githubclient "github.com/google/go-github/v50/github"
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
	Client       *githubclient.Client
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
	var client *githubclient.Client
	{
		client = githubclient.NewClient(nil)
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
	//Create application registration if needed
	secret, err := g.createApp(config, ctx, oldConnector)
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

func (g *Github) createApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderSecret, error) {
	// get app
	app, _, err := g.Client.Apps.Get(ctx, config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return provider.ProviderSecret{}, microerror.Mask(err)
		}
		//create the app
		appConfig, _, err := g.Client.Apps.CompleteAppManifest(ctx, "")
		if err != nil {
			return provider.ProviderSecret{}, microerror.Mask(err)
		}
		return provider.ProviderSecret{
			ClientSecret: appConfig.GetClientSecret(),
			ClientId:     appConfig.GetClientID(),
			EndDateTime:  time.Now().AddDate(0, 6, 0),
		}, nil
	}

	if err := g.checkForUpdate(app, config); err != nil {
		return provider.ProviderSecret{}, microerror.Mask(err)
	}
	// Retrieve old id and secret
	oldID, oldSecret, err := getSecretFromConfig(oldConnector.Config)
	if err != nil {
		return provider.ProviderSecret{}, microerror.Mask(err)
	}

	return provider.ProviderSecret{
		EndDateTime:  app.GetCreatedAt().AddDate(0, 6, 0),
		ClientId:     oldID,
		ClientSecret: oldSecret,
	}, nil
}

func (g *Github) checkForUpdate(app *githubclient.App, config provider.AppConfig) error {
	//Compute if update needed
	//Somehow communicate a needed update
	return nil
}
