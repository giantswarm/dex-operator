package azure

import (
	"context"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"

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
	Owner                 = "giantswarm" //TODO: make this variable so it can be used for customer too
)

type Azure struct {
	Name     string
	Auther   *azauth.AzureIdentityAuthenticationProvider
	TenantID string
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
	return &Azure{
		Name:     ProviderName,
		Auther:   auth,
		TenantID: tenantID,
	}, nil
}

func (a *Azure) CreateApp(config provider.AppConfig, ctx context.Context) (dex.Connector, error) {

	adapter, err := msgraphsdk.NewGraphRequestAdapter(a.Auther)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	client := msgraphsdk.NewGraphServiceClient(adapter)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	createdApp, err := client.Applications().Post(ctx, getAppRequestBody(config), nil)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	createdSecret, err := client.ApplicationsById(*createdApp.GetId()).AddPassword().Post(ctx, getSecretRequestBody(config), nil)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}

	return dex.Connector{
		Type: ProviderConnectorType,
		ID:   fmt.Sprintf("%s-%s", Owner, ProviderName),
		Name: fmt.Sprintf("%s for %s", ProviderConnectorType, Owner),
		Config: &microsoft.Config{
			ClientID:     *createdSecret.GetKeyId(),
			ClientSecret: *createdSecret.GetSecretText(),
			RedirectURI:  config.RedirectURI,
			Tenant:       a.TenantID,
		},
	}, nil
}

func (a *Azure) DeleteApp(name string) error {
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
