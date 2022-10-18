package azure

import (
	"context"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dexidp/dex/connector/microsoft"
	"github.com/giantswarm/microerror"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applications/item/addpassword"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

const (
	ProviderName          = "ad"
	ProviderConnectorType = "microsoft"
	TenantIDKey           = "tenant-id"
	ClientIDKey           = "client-id"
	ClientSecretKey       = "client-secret"
)

type Azure struct {
	Name     string
	Client   *msgraphsdk.GraphServiceClient
	Owner    string
	TenantID string
	Type     string
}

func New(p provider.ProviderCredential) (*Azure, error) {
	var tenantID, clientID, clientSecret string
	{
		if tenantID = p.Credentials[TenantIDKey]; tenantID == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", TenantIDKey)
		}
		if clientID = p.Credentials[ClientIDKey]; clientID == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientIDKey)
		}
		if clientSecret = p.Credentials[ClientSecretKey]; clientSecret == "" {
			return nil, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientSecretKey)
		}
	}
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	auth, err := azauth.NewAzureIdentityAuthenticationProviderWithScopes(cred, []string{"User.Read"})
	if err != nil {
		return nil, microerror.Mask(err)
	}
	adapter, err := msgraphsdk.NewGraphRequestAdapter(auth)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	client := msgraphsdk.NewGraphServiceClient(adapter)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	return &Azure{
		Name:     fmt.Sprintf("%s-%s", p.Owner, p.Name),
		Type:     ProviderConnectorType,
		Client:   client,
		Owner:    p.Owner,
		TenantID: tenantID,
	}, nil
}

func (a *Azure) GetName() string {
	return a.Name
}

func (a *Azure) GetType() string {
	return a.Type
}

func (a *Azure) GetOwner() string {
	return a.Owner
}

func (a *Azure) CreateApp(config provider.AppConfig, ctx context.Context) (dex.Connector, error) {
	createdApp, err := a.Client.Applications().Post(ctx, getAppRequestBody(config), nil)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	createdSecret, err := a.Client.ApplicationsById(*createdApp.GetId()).AddPassword().Post(ctx, getSecretRequestBody(config), nil)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	return dex.Connector{
		Type: a.Type,
		ID:   a.Name,
		Name: key.GetConnectorDescription(ProviderConnectorType, a.Owner),
		Config: &microsoft.Config{
			ClientID:     *createdSecret.GetKeyId(),
			ClientSecret: *createdSecret.GetSecretText(),
			RedirectURI:  config.RedirectURI,
			Tenant:       a.TenantID,
		},
	}, nil
}

func (a *Azure) DeleteApp(name string) error {
	// TODO this needs to be the id. Where does the id come from?
	if err := a.Client.ApplicationsById(name).Delete(context.Background(), nil); err != nil {
		//TODO catch not found case
		return microerror.Mask(err)
	}
	return nil
}

func getAppRequestBody(config provider.AppConfig) *models.Application {
	web := models.NewWebApplication()
	web.SetRedirectUris([]string{config.RedirectURI})

	app := models.NewApplication()
	app.SetDisplayName(&config.Name)
	app.SetWeb(web)

	return app
}

func getSecretRequestBody(config provider.AppConfig) *addpassword.AddPasswordPostRequestBody {
	keyCredential := models.NewPasswordCredential()
	keyCredential.SetDisplayName(&config.Name)

	secret := addpassword.NewAddPasswordPostRequestBody()
	secret.SetPasswordCredential(keyCredential)

	return secret
}
