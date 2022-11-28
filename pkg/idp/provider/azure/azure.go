package azure

import (
	"context"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"strings"
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

func New(p provider.ProviderCredential, log *logr.Logger) (*Azure, error) {
	var tenantID, clientID, clientSecret string
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
		Log:      log,
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

func (a *Azure) CreateOrUpdateApp(config provider.AppConfig, ctx context.Context, oldConnector dex.Connector) (dex.Connector, error) {
	// Create or update application registration
	id, err := a.createOrUpdateApplication(config, ctx)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}

	// Retrieve old secret
	oldSecret, err := getSecretFromConfig(oldConnector.Config)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}

	//Create or update secret
	clientId, clientSecret, err := a.createOrUpdateSecret(id, config, ctx, oldSecret)
	if err != nil {
		return dex.Connector{}, microerror.Mask(err)
	}

	// Write to connector
	connectorConfig := &microsoft.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
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

func (a *Azure) createOrUpdateApplication(config provider.AppConfig, ctx context.Context) (string, error) {
	app, err := a.GetApp(config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return "", microerror.Mask(err)
		}
		// Create app if it does not exist
		app, err = a.Client.Applications().Post(ctx, getAppCreateRequestBody(config), nil)
		if err != nil {
			return "", microerror.Maskf(requestFailedError, printOdataError(err))
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
			return "", microerror.Maskf(requestFailedError, printOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Updated %s app %s for %s in microsoft ad tenant %s", a.Type, config.Name, a.Owner, a.TenantID))
	}
	return *id, nil
}

func (a *Azure) createOrUpdateSecret(id string, config provider.AppConfig, ctx context.Context, oldSecret string) (string, string, error) {

	app, err := a.Client.ApplicationsById(id).Get(ctx, nil)
	if err != nil {
		return "", "", microerror.Maskf(requestFailedError, printOdataError(err))
	}

	var needsCreation bool

	// Secret needs to be created if no secret can be found
	secret, err := a.GetSecret(app, config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return "", "", microerror.Mask(err)
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
			return "", "", microerror.Maskf(requestFailedError, printOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Removed secret %v of %s app %s for %s in microsoft ad tenant %s", secret.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
		needsCreation = true
	}

	// Create secret if it does not exist
	if needsCreation {
		secret, err = a.Client.ApplicationsById(id).AddPassword().Post(ctx, getSecretCreateRequestBody(config), nil)
		if err != nil {
			return "", "", microerror.Maskf(requestFailedError, printOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Created secret %v of %s app %s for %s in microsoft ad tenant %s", secret.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
	}
	var clientSecret, clientId string
	{
		//Get connector data
		if secret.GetSecretText() == nil || *secret.GetSecretText() == "" {
			clientSecret = oldSecret
		} else {
			clientSecret = *secret.GetSecretText()
		}
		if app.GetAppId() == nil || *app.GetAppId() == "" {
			return "", "", microerror.Maskf(notFoundError, "Could not find App ID of app %s.", config.Name)
		}
		clientId = *app.GetAppId()
	}
	return clientId, clientSecret, nil
}

func secretExpired(secret models.PasswordCredentialable) bool {
	bestBefore := secret.GetEndDateTime()
	if bestBefore == nil {
		return true
	}
	if bestBefore.Before(time.Now().Add(24 * time.Hour)) {
		return true
	}
	return false
}

func secretChanged(secret models.PasswordCredentialable, oldSecret string) bool {
	hint := secret.GetHint()
	if hint == nil {
		return true
	}
	if !strings.HasPrefix(oldSecret, *hint) {
		return true
	}
	return false
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
		result, err := a.Client.Applications().Get(context.Background(), getAppGetRequestConfig(name))
		if err != nil {
			return microerror.Maskf(requestFailedError, printOdataError(err))
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

func (a *Azure) GetSecret(app models.Applicationable, name string) (models.PasswordCredentialable, error) {
	for _, c := range app.GetPasswordCredentials() {
		if credentialName := c.GetDisplayName(); credentialName != nil {
			if *credentialName == name {
				return c, nil
			}
		}
	}
	return nil, microerror.Maskf(notFoundError, "Did not find credential %s.", name)
}

func getSecretFromConfig(config string) (string, error) {
	if config == "" {
		return "", nil
	}
	configData := []byte(config)
	connectorConfig := &microsoft.Config{}
	if err := yaml.Unmarshal(configData, connectorConfig); err != nil {
		return "", microerror.Mask(err)
	}
	return connectorConfig.ClientSecret, nil
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
