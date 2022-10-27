package azure

import (
	"context"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dexidp/dex/connector/microsoft"
	"github.com/giantswarm/microerror"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
	"github.com/microsoftgraph/msgraph-sdk-go/applications/item/addpassword"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

const (
	ProviderName          = "ad"
	ProviderConnectorType = "microsoft"
	TenantIDKey           = "tenant-id"
	ClientIDKey           = "client-id"
	ClientSecretKey       = "client-secret"
	PermissionType        = "Scope"
	DefaultName           = "giantswarm-dex"
)

func ProviderScope() []string {
	return []string{"https://graph.microsoft.com/.default"}
}

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
		if p.Name == "" {
			return nil, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
		}
		if p.Owner == "" {
			return nil, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
		}
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
	auth, err := azauth.NewAzureIdentityAuthenticationProviderWithScopes(cred, ProviderScope())
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
	permissions, err := a.getPermissions()
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}

	createdApp, err := a.Client.Applications().Post(ctx, getAppCreateRequestBody(config, permissions), nil)
	if err != nil {
		return dex.Connector{}, microerror.Maskf(requestFailedError, printOdataError(err))
	}
	id := createdApp.GetId()
	if id == nil {
		return dex.Connector{}, microerror.Maskf(notFoundError, "Could not find ID of app %s.", config.Name)
	}
	createdSecret, err := a.Client.ApplicationsById(*id).AddPassword().Post(ctx, getSecretCreateRequestBody(config), nil)
	if err != nil {
		return dex.Connector{}, microerror.Maskf(requestFailedError, printOdataError(err))
	}
	clientID := createdSecret.GetKeyId()
	if clientID == nil {
		return dex.Connector{}, microerror.Maskf(notFoundError, "Could not find client id for app %s.", config.Name)
	}
	clientSecret := createdSecret.GetSecretText()
	if clientSecret == nil {
		return dex.Connector{}, microerror.Maskf(notFoundError, "Could not find client secret for app %s.", config.Name)
	}
	return dex.Connector{
		Type: a.Type,
		ID:   a.Name,
		Name: key.GetConnectorDescription(ProviderConnectorType, a.Owner),
		Config: &microsoft.Config{
			ClientID:     *clientID,
			ClientSecret: *clientSecret,
			RedirectURI:  config.RedirectURI,
			Tenant:       a.TenantID,
		},
	}, nil
}

func (a *Azure) DeleteApp(name string) error {
	appID, err := a.GetAppID(name)
	if err != nil {
		if IsNotExist(err) {
			// already deleted case
			return nil
		}
		return microerror.Mask(err)
	}
	if err := a.Client.ApplicationsById(appID).Delete(context.Background(), nil); err != nil {
		return microerror.Maskf(requestFailedError, printOdataError(err))
	}
	return nil
}

func (a *Azure) GetAppID(name string) (string, error) {
	app, err := a.GetApp(name)
	if err != nil {
		return "", microerror.Mask(err)
	}
	id := app.GetId()
	if id == nil {
		return "", microerror.Maskf(notFoundError, "Could not find ID of app %s.", name)
	}
	return *id, nil
}

func (a *Azure) GetApp(name string) (models.Applicationable, error) {
	result, err := a.Client.Applications().Get(context.Background(), getAppGetRequestConfig(name))
	if err != nil {
		return nil, microerror.Maskf(requestFailedError, printOdataError(err))
	}
	count := result.GetOdataCount()
	if *count == 0 {
		return nil, microerror.Maskf(notFoundError, "No application with name %s exists.", name)
	} else if *count != 1 {
		return nil, microerror.Maskf(notFoundError, "Expected 1 application %s, got %v.", name, count)
	}
	appList := result.GetValue()
	if len(appList) != 1 {
		return nil, microerror.Maskf(notFoundError, "Expected 1 application %s, got %v.", name, len(appList))
	}
	return appList[0], nil
}

func (a *Azure) getPermissions() ([]models.RequiredResourceAccessable, error) {
	result, err := a.GetApp(DefaultName)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	var resourceAccess []models.ResourceAccessable
	t := PermissionType
	for _, rra := range result.GetRequiredResourceAccess() {
		for _, ra := range rra.GetResourceAccess() {
			if ra.GetType() == &t {
				resourceAccess = append(resourceAccess, ra)
			}
		}
	}
	requiredResourceaccess := models.NewRequiredResourceAccess()
	requiredResourceaccess.SetResourceAccess(resourceAccess)
	return []models.RequiredResourceAccessable{requiredResourceaccess}, nil
}

func getAppGetRequestConfig(name string) *applications.ApplicationsRequestBuilderGetRequestConfiguration {
	headers := map[string]string{
		"ConsistencyLevel": "eventual",
	}
	requestFilter := fmt.Sprintf("displayName eq '%s'", name)
	requestCount := true
	requestTop := int32(1)

	requestParameters := &applications.ApplicationsRequestBuilderGetQueryParameters{
		Filter:  &requestFilter,
		Count:   &requestCount,
		Top:     &requestTop,
		Orderby: []string{"displayName"},
	}
	return &applications.ApplicationsRequestBuilderGetRequestConfiguration{
		Headers:         headers,
		QueryParameters: requestParameters,
	}
}

func getAppCreateRequestBody(config provider.AppConfig, permissions []models.RequiredResourceAccessable) *models.Application {
	web := models.NewWebApplication()
	web.SetRedirectUris([]string{config.RedirectURI})
	app := models.NewApplication()
	app.SetDisplayName(&config.Name)
	app.SetWeb(web)
	app.SetRequiredResourceAccess(permissions)

	return app
}

func getSecretCreateRequestBody(config provider.AppConfig) *addpassword.AddPasswordPostRequestBody {
	keyCredential := models.NewPasswordCredential()
	keyCredential.SetDisplayName(&config.Name)

	validUntil := time.Now().AddDate(0, key.SecretValidityMonths, 0)
	keyCredential.SetEndDateTime(&validUntil)

	secret := addpassword.NewAddPasswordPostRequestBody()
	secret.SetPasswordCredential(keyCredential)

	return secret
}
