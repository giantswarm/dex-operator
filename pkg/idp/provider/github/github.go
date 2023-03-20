package github

import (
	"context"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"net/http"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
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
	AppIDKey              = "app-id"
	PrivateKeyKey         = "private-key"
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
	id           string
	secret       string
}

type Config struct {
	Organization string
	Team         string
	AppID        int64
	PrivateKey   []byte
	ClientID     string
	ClientSecret string
}

func New(p provider.ProviderCredential, log *logr.Logger) (*Github, error) {

	// get configuration from credentials
	c, err := newGithubConfig(p, log)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// get the client
	itr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, c.AppID, c.PrivateKey)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	client := githubclient.NewClient(&http.Client{Transport: itr})

	return &Github{
		Name:         key.GetProviderName(p.Owner, p.Name),
		Log:          log,
		Type:         ProviderConnectorType,
		Client:       client,
		Owner:        p.Owner,
		Organization: c.Organization,
		Team:         c.Team,
		id:           c.ClientID,
		secret:       c.ClientSecret,
	}, nil
}

func newGithubConfig(p provider.ProviderCredential, log *logr.Logger) (Config, error) {
	if log == nil {
		return Config{}, microerror.Maskf(invalidConfigError, "Logger must not be empty.")
	}
	if p.Name == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
	}
	if p.Owner == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
	}

	var organization, team, clientSecret, clientID string
	{
		if organization = p.Credentials[OrganizationKey]; organization == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", OrganizationKey)
		}
		if team = p.Credentials[TeamKey]; team == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", TeamKey)
		}
		if clientSecret = p.Credentials[ClientSecretKey]; clientSecret == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientSecretKey)
		}
		if clientID = p.Credentials[ClientIDKey]; clientID == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientIDKey)
		}
	}

	var appID int
	{
		var err error
		if appIDvalue := p.Credentials[AppIDKey]; appIDvalue == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", AppIDKey)
		} else {
			if appID, err = strconv.Atoi(appIDvalue); err != nil {
				return Config{}, microerror.Maskf(invalidConfigError, "%s is not a valid value for %s: %v", appIDvalue, AppIDKey, err)
			}
		}
	}

	var privateKey []byte
	{
		if privateKeyValue := p.Credentials[PrivateKeyKey]; privateKeyValue == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", PrivateKeyKey)
		} else {
			privateKey = []byte(privateKeyValue)
		}
	}

	return Config{
		Organization: organization,
		Team:         team,
		AppID:        int64(appID),
		PrivateKey:   privateKey,
		ClientSecret: clientSecret,
		ClientID:     clientID,
	}, nil
}

func (g *Github) GetName() string {
	return g.Name
}

func (g *Github) GetProviderName() string {
	return ProviderName
}

func (g *Github) GetType() string {
	return g.Type
}

func (g *Github) GetOwner() string {
	return g.Owner
}

func (g *Github) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderApp, error) {
	secret, err := g.createOrUpdateSecret(config, ctx, oldConnector)
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
	// get authenticated app
	_, _, err := g.Client.Apps.Get(ctx, "")
	if err != nil {
		return microerror.Mask(err)
	}
	//TODO: remove redirect URI for the affected app
	return nil
}

func (g *Github) createOrUpdateSecret(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderSecret, error) {
	// get authenticated app, check if the callback URI is present
	app, _, err := g.Client.Apps.Get(ctx, "")
	if err != nil {
		return provider.ProviderSecret{}, microerror.Mask(err)
	}
	if !callbackURIPresent(app, config) {
		//We return here since we can not automatically set the URI
		return provider.ProviderSecret{}, microerror.Maskf(missingCallbackURIError, fmt.Sprintf("Callback URI %s is not registered in %s app %s for %s in organization %s.", config.RedirectURI, g.Type, app.GetSlug(), g.Owner, g.Organization))
	}
	if g.UpdateNeeded(app, config) {
		//We return here since we can not set the update
		return provider.ProviderSecret{}, microerror.Maskf(missingCallbackURIError, fmt.Sprintf("%s app %s for %s in github organization %s needs update.", g.Type, app.GetSlug(), g.Owner, g.Organization))
	}
	return g.getSecret(app, oldConnector)
}

func (g *Github) getSecret(app *githubclient.App, oldConnector dex.Connector) (provider.ProviderSecret, error) {
	var err error
	var endDateTime time.Time
	var clientID, clientSecret string
	{
		// Retrieve old id and secret
		clientID, clientSecret, err = getSecretFromConfig(oldConnector.Config)
		if err != nil {
			return provider.ProviderSecret{}, microerror.Mask(err)
		}
		//currently the authenticated app represents the MC dex app
		//WC case can be handled by adding extra callback URIs to the authenticated app
		if clientID != g.id || clientSecret != g.secret {
			clientID = g.id
			clientSecret = g.secret
		}
		endDateTime = app.GetCreatedAt().AddDate(0, 6, 0)
	}

	return provider.ProviderSecret{
		EndDateTime:  endDateTime,
		ClientId:     clientID,
		ClientSecret: clientSecret,
	}, nil
}

func (g *Github) UpdateNeeded(app *githubclient.App, config provider.AppConfig) bool {
	if permissionsUpdateNeeded(app) {
		g.Log.Info(fmt.Sprintf("Permissions of %s app %s for %s in github organization %s needs update.", g.Type, app.GetSlug(), g.Owner, g.Organization))
		return true
	}
	return false
}

func permissionsUpdateNeeded(app *githubclient.App) bool {
	permissions := app.GetPermissions()
	return !(permissions.GetEmails() == "read" && permissions.GetMembers() == "read")
}

func callbackURIPresent(app *githubclient.App, config provider.AppConfig) bool {
	//TODO: check if callback URL is present
	return true
}

func (g *Github) GetCredentialsForAuthenticatedApp(config provider.AppConfig) (string, error) {
	// TODO: manifest flow
	c := Config{}
	return fmt.Sprintf(`client-id: %s
client-secret: %s
organization: %s
team: %s
app-id: %v
private-key: %s`, c.ClientID, c.ClientSecret, c.Organization, c.Team, c.AppID, c.PrivateKey), nil
}
func (g *Github) CleanCredentialsForAuthenticatedApp(config provider.AppConfig) error {
	// TODO open for deletion?
	return nil
}
