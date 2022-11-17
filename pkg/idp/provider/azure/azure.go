package azure

import (
	"context"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dexidp/dex/connector/microsoft"
	"github.com/giantswarm/microerror"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"gopkg.in/yaml.v2"
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
	var client *msgraphsdk.GraphServiceClient
	{
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
		client = msgraphsdk.NewGraphServiceClient(adapter)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}
	return &Azure{
		Name:     key.GetProviderName(p.Owner, p.Name),
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

func (a *Azure) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, currentConnector dex.Connector) (dex.Connector, error) {
	var clientId, clientSecret *string
	{
		// Create or update application registration
		app, err := a.createOrUpdateApplication(config, ctx)
		if err != nil {
			return dex.Connector{}, microerror.Mask(err)
		}
		id := app.GetId()
		if id == nil {
			return dex.Connector{}, microerror.Maskf(notFoundError, "Could not find ID of app %s.", config.Name)
		}
		//Create or update secret
		secret, err := a.createOrUpdateSecret(*id, config, ctx)
		if err != nil {
			return dex.Connector{}, microerror.Mask(err)
		}

		//Get connector data
		clientSecret = secret.GetSecretText()
		if clientSecret == nil {
			return dex.Connector{}, microerror.Maskf(notFoundError, "Could not find client secret for app %s.", config.Name)
		}
		clientId = app.GetAppId()
		if clientId == nil {
			return dex.Connector{}, microerror.Maskf(notFoundError, "Could not find App ID of app %s.", config.Name)
		}
	}
	// Write to connector
	connectorConfig := &microsoft.Config{
		ClientID:     *clientId,
		ClientSecret: *clientSecret,
		RedirectURI:  config.RedirectURI,
		Tenant:       a.TenantID,
	}
	data, err := yaml.Marshal(connectorConfig)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}
	return dex.Connector{
		Type:   a.Type,
		ID:     a.Name,
		Name:   key.GetConnectorDescription(ProviderConnectorType, a.Owner),
		Config: string(data[:]),
	}, nil
}

func (a *Azure) createOrUpdateApplication(config provider.AppConfig, ctx context.Context) (models.Applicationable, error) {
	app, err := a.GetApp(config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return nil, microerror.Mask(err)
		}
		// Create app if it does not exist
		app, err = a.Client.Applications().Post(ctx, getAppCreateRequestBody(config), nil)
		if err != nil {
			return nil, microerror.Maskf(requestFailedError, printOdataError(err))
		}
	}
	//Update if needed
	needsUpdate, patch, err := a.computeAppUpdatePatch(config, app, ctx)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	if needsUpdate {
		id := app.GetId()
		if id == nil {
			return nil, microerror.Maskf(notFoundError, "Could not find ID of app %s.", config.Name)
		}
		app, err = a.Client.ApplicationsById(*id).Patch(ctx, patch, nil)
		if err != nil {
			return nil, microerror.Maskf(requestFailedError, printOdataError(err))
		}
	}
	return app, nil
}

func (a *Azure) computeAppUpdatePatch(config provider.AppConfig, app models.Applicationable, ctx context.Context) (bool, models.Applicationable, error) {
	appPatch := models.NewApplication()
	appNeedsUpdate := false
	{
		needsUpdate, patch, err := a.computePermissionsUpdatePatch(app, ctx)
		if err != nil {
			return false, nil, microerror.Mask(err)
		}
		if needsUpdate {
			appNeedsUpdate = true
			appPatch.SetRequiredResourceAccess(patch)
		}
	}
	if needsUpdate, patch := a.computeRedirectURIUpdatePatch(app, config, ctx); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetWeb(patch)
	}
	return appNeedsUpdate, appPatch, nil
}

func (a *Azure) computePermissionsUpdatePatch(app models.Applicationable, ctx context.Context) (bool, []models.RequiredResourceAccessable, error) {
	var original, patch []models.RequiredResourceAccessable
	{
		original = app.GetRequiredResourceAccess()

		parentApp, err := a.GetApp(DefaultName)
		if err != nil {
			return false, nil, microerror.Mask(err)
		}
		patch = getPermissionCreateRequestBody(parentApp)
	}
	if reflect.DeepEqual(original, patch) {
		return false, nil, nil
	}
	return true, patch, nil
}

func (a *Azure) computeRedirectURIUpdatePatch(app models.Applicationable, config provider.AppConfig, ctx context.Context) (bool, models.WebApplicationable) {
	var original, patch models.WebApplicationable
	{
		original = app.GetWeb()
		patch = getRedirectURIsRequestBody([]string{config.RedirectURI})
	}
	if reflect.DeepEqual(original, patch) {
		return false, nil
	}
	return true, patch
}

func (a *Azure) createOrUpdateSecret(id string, config provider.AppConfig, ctx context.Context) (models.PasswordCredentialable, error) {
	createdSecret, err := a.Client.ApplicationsById(id).AddPassword().Post(ctx, getSecretCreateRequestBody(config), nil)
	if err != nil {
		return nil, microerror.Maskf(requestFailedError, printOdataError(err))
	}
	return createdSecret, nil
}

func (a *Azure) DeleteApp(name string, ctx context.Context) error {
	appID, err := a.GetAppID(name)
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return microerror.Mask(err)
	}
	if err := a.Client.ApplicationsById(appID).Delete(ctx, nil); err != nil {
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
