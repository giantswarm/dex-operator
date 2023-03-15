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
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applications/item/removepassword"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"gopkg.in/yaml.v2"
)

type Azure struct {
	Name     string
	Client   *msgraphsdk.GraphServiceClient
	Log      *logr.Logger
	Owner    string
	TenantID string
	Type     string
}

type config struct {
	tenantID     string
	clientID     string
	clientSecret string
}

func New(p provider.ProviderCredential, log *logr.Logger) (*Azure, error) {

	// get configuration from credentials
	c, err := newAzureConfig(p, log)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var client *msgraphsdk.GraphServiceClient
	{
		cred, err := azidentity.NewClientSecretCredential(c.tenantID, c.clientID, c.clientSecret, nil)
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
		Log:      log,
		Type:     ProviderConnectorType,
		Client:   client,
		Owner:    p.Owner,
		TenantID: c.tenantID,
	}, nil
}

func newAzureConfig(p provider.ProviderCredential, log *logr.Logger) (config, error) {
	if log == nil {
		return config{}, microerror.Maskf(invalidConfigError, "Logger must not be empty.")
	}
	if p.Name == "" {
		return config{}, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
	}
	if p.Owner == "" {
		return config{}, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
	}

	var tenantID, clientID, clientSecret string
	{
		if tenantID = p.Credentials[TenantIDKey]; tenantID == "" {
			return config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", TenantIDKey)
		}
		if clientID = p.Credentials[ClientIDKey]; clientID == "" {
			return config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientIDKey)
		}
		if clientSecret = p.Credentials[ClientSecretKey]; clientSecret == "" {
			return config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientSecretKey)
		}
	}

	return config{
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
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

func (a *Azure) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (provider.ProviderApp, error) {
	// Create or update application registration
	id, err := a.createOrUpdateApplication(config, ctx)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	// Retrieve old secret
	oldSecret, err := getSecretFromConfig(oldConnector.Config)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	//Create or update secret
	secret, err := a.createOrUpdateSecret(id, config, ctx, oldSecret)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}

	// Write to connector
	connectorConfig := &microsoft.Config{
		ClientID:     secret.ClientId,
		ClientSecret: secret.ClientSecret,
		RedirectURI:  config.RedirectURI,
		Tenant:       a.TenantID,
	}
	data, err := yaml.Marshal(connectorConfig)
	if err != nil {
		return provider.ProviderApp{}, microerror.Mask(err)
	}
	return provider.ProviderApp{
		Connector: dex.Connector{
			Type:   a.Type,
			ID:     a.Name,
			Name:   key.GetConnectorDescription(ProviderConnectorType, a.Owner),
			Config: string(data[:]),
		},
		SecretEndDateTime: secret.EndDateTime,
	}, nil
}

func (a *Azure) createOrUpdateApplication(config provider.AppConfig, ctx context.Context) (string, error) {
	app, err := a.GetApp(config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return "", microerror.Mask(err)
		}
		// Create app if it does not exist
		app, err = a.Client.Applications().Post(ctx, getAppCreateRequestBody(config), nil)
		if err != nil {
			return "", microerror.Maskf(requestFailedError, PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Created %s app %s for %s in microsoft ad tenant %s", a.Type, config.Name, a.Owner, a.TenantID))
	}

	// We need to get the dex parent app to determine which permissions should be set.
	// Because microsoft graph api does not allow for checking the permissions scope (in human readable form) of a given app or setting the scope via anything else than
	// hardcoding the permissions ids, we instead set them based on an existing app in the tenant.
	// This way permissions can be set and revoked for child apps easily and we ensure that the right permissions are set.
	parentApp, err := a.GetApp(DefaultName)
	if err != nil {
		return "", microerror.Mask(err)
	}
	id := app.GetId()
	if id == nil {
		return "", microerror.Maskf(notFoundError, "Could not find ID of app %s.", config.Name)
	}

	//Update if needed
	if needsUpdate, patch := a.computeAppUpdatePatch(config, app, parentApp); needsUpdate {
		_, err = a.Client.ApplicationsById(*id).Patch(ctx, patch, nil)
		if err != nil {
			return "", microerror.Maskf(requestFailedError, PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Updated %s app %s for %s in microsoft ad tenant %s", a.Type, config.Name, a.Owner, a.TenantID))
	}
	return *id, nil
}

func (a *Azure) createOrUpdateSecret(id string, config provider.AppConfig, ctx context.Context, oldSecret string) (provider.ProviderSecret, error) {

	app, err := a.Client.ApplicationsById(id).Get(ctx, nil)
	if err != nil {
		return provider.ProviderSecret{}, microerror.Maskf(requestFailedError, PrintOdataError(err))
	}

	var needsCreation bool

	// Secret needs to be created if no secret can be found
	secret, err := GetSecret(app, config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return provider.ProviderSecret{}, microerror.Mask(err)
		}
		needsCreation = true
	}

	// Check if we already have a key
	keyPresent := oldSecret != ""

	// We delete the secret in case it exists and is expired or in case we do not have the key anymore
	if !needsCreation && (!keyPresent || secretExpired(secret) || secretChanged(secret, oldSecret)) {
		requestBody := removepassword.NewRemovePasswordPostRequestBody()
		requestBody.SetKeyId(secret.GetKeyId())

		err = a.Client.ApplicationsById(id).RemovePassword().Post(context.Background(), requestBody, nil)
		if err != nil {
			return provider.ProviderSecret{}, microerror.Maskf(requestFailedError, PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Removed secret %v of %s app %s for %s in microsoft ad tenant %s", secret.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
		needsCreation = true
	}

	// Create secret if it does not exist
	if needsCreation {
		secret, err = a.Client.ApplicationsById(id).AddPassword().Post(ctx, GetSecretCreateRequestBody(config.Name, key.SecretValidityMonths), nil)
		if err != nil {
			return provider.ProviderSecret{}, microerror.Maskf(requestFailedError, PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Created secret %v of %s app %s for %s in microsoft ad tenant %s", secret.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
	}
	return getAzureSecret(secret, app, oldSecret)
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
		return microerror.Maskf(requestFailedError, PrintOdataError(err))
	}
	a.Log.Info(fmt.Sprintf("Deleted %s app %s for %s in microsoft ad tenant %s", a.Type, name, a.Owner, a.TenantID))
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
	var appList []models.Applicationable

	o := func() error {
		result, err := a.Client.Applications().Get(context.Background(), GetAppGetRequestConfig(name))
		if err != nil {
			return microerror.Maskf(requestFailedError, PrintOdataError(err))
		}
		count := result.GetOdataCount()
		if *count == 0 {
			return microerror.Maskf(notFoundError, "No application with name %s exists.", name)
		} else if *count != 1 {
			return microerror.Maskf(notFoundError, "Expected 1 application %s, got %v.", name, count)
		}
		appList = result.GetValue()
		if len(appList) != 1 {
			return microerror.Maskf(notFoundError, "Expected 1 application %s, got %v.", name, len(appList))
		}
		return nil
	}
	b := backoff.NewMaxRetries(20, 3*time.Second)
	err := backoff.Retry(o, b)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	return appList[0], nil
}

func (a *Azure) computeAppUpdatePatch(config provider.AppConfig, app models.Applicationable, parentApp models.Applicationable) (bool, models.Applicationable) {
	appPatch := models.NewApplication()
	appNeedsUpdate := false

	if needsUpdate, patch := computePermissionsUpdatePatch(app, parentApp); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetRequiredResourceAccess(patch)
		a.Log.Info(fmt.Sprintf("Permissions of %s app %s for %s in microsoft ad tenant %s need update.", a.Type, config.Name, a.Owner, a.TenantID))
	}

	if needsUpdate, patch := computeRedirectURIUpdatePatch(app, config); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetWeb(patch)
		a.Log.Info(fmt.Sprintf("Redirect URI of %s app %s for %s in microsoft ad tenant %s need update.", a.Type, config.Name, a.Owner, a.TenantID))
	}

	if needsUpdate, patch := computeClaimsUpdatePatch(app); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetOptionalClaims(patch)
		a.Log.Info(fmt.Sprintf("Claims of %s app %s for %s in microsoft ad tenant %s need update.", a.Type, config.Name, a.Owner, a.TenantID))
	}

	return appNeedsUpdate, appPatch
}
