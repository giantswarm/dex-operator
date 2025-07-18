package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dexidp/dex/connector/microsoft"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/skratchdot/open-golang/open"
	"gopkg.in/yaml.v2"
)

var _ provider.Provider = (*Azure)(nil)

func (a *Azure) SupportsServiceCredentialRenewal() bool {
	return true
}

func (a *Azure) ShouldRotateServiceCredentials(ctx context.Context, config provider.AppConfig) (bool, error) {
	const renewalThreshold = 30 * 24 * time.Hour

	appName := key.GetDexOperatorName(a.managementClusterName)

	app, err := a.GetApp(appName)
	if err != nil {
		a.Log.Info("Could not get Azure app for credential expiry check, assuming renewal needed",
			"app", appName, "error", err)
		return true, nil
	}

	// Find the current secret for this app
	secret, err := GetSecret(app, appName)
	if err != nil {
		a.Log.Info("Could not get Azure secret for expiry check, assuming renewal needed",
			"app", appName, "error", err)
		return true, nil
	}

	if endDateTime := secret.GetEndDateTime(); endDateTime != nil {
		timeUntilExpiry := time.Until(*endDateTime)
		a.Log.Info("Azure credential expiry check",
			"app", appName,
			"expiry", *endDateTime,
			"time_until_expiry", timeUntilExpiry)

		return timeUntilExpiry < renewalThreshold, nil
	}

	a.Log.Info("No expiry time found for Azure credential, assuming renewal needed", "app", appName)
	return true, nil
}

// RotateServiceCredentials implements SelfRenewalProvider
func (a *Azure) RotateServiceCredentials(ctx context.Context, config provider.AppConfig) (map[string]string, error) {
	a.Log.Info("Rotating Azure service credentials", "app", config.Name)

	credentials, err := a.GetCredentialsForAuthenticatedApp(config)
	if err != nil {
		return nil, microerror.Maskf(requestFailedError, "Failed to rotate Azure credentials: %v", err)
	}

	a.Log.Info("Successfully rotated Azure service credentials", "app", config.Name)
	return credentials, nil
}

type Azure struct {
	Name                  string
	Description           string
	Client                *msgraphsdk.GraphServiceClient
	Log                   logr.Logger
	Owner                 string
	TenantID              string
	Type                  string
	clientSecret          string
	managementClusterName string
}

type Config struct {
	TenantID     string
	ClientID     string
	ClientSecret string
}

func New(config provider.ProviderConfig) (*Azure, error) {
	// get configuration from credentials
	c, err := newAzureConfig(config.Credential, config.Log)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var client *msgraphsdk.GraphServiceClient
	{
		cred, err := azidentity.NewClientSecretCredential(c.TenantID, c.ClientID, c.ClientSecret, nil)
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
		Name:                  key.GetProviderName(config.Credential.Owner, config.Credential.Name),
		Description:           config.Credential.GetConnectorDescription(ProviderDisplayName),
		Log:                   config.Log,
		Type:                  ProviderConnectorType,
		Client:                client,
		Owner:                 config.Credential.Owner,
		TenantID:              c.TenantID,
		clientSecret:          c.ClientSecret,
		managementClusterName: config.ManagementClusterName,
	}, nil
}

func newAzureConfig(p provider.ProviderCredential, log logr.Logger) (Config, error) {
	if (logr.Logger{}) == log {
		return Config{}, microerror.Maskf(invalidConfigError, "Logger must not be empty.")
	}
	if p.Name == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential name must not be empty.")
	}
	if p.Owner == "" {
		return Config{}, microerror.Maskf(invalidConfigError, "Credential owner must not be empty.")
	}

	var tenantID, clientID, clientSecret string
	{
		if tenantID = p.Credentials[TenantIDKey]; tenantID == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", TenantIDKey)
		}
		if clientID = p.Credentials[ClientIDKey]; clientID == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientIDKey)
		}
		if clientSecret = p.Credentials[ClientSecretKey]; clientSecret == "" {
			return Config{}, microerror.Maskf(invalidConfigError, "%s must not be empty.", ClientSecretKey)
		}
	}

	return Config{
		TenantID:     tenantID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}, nil
}

func (a *Azure) GetName() string {
	return a.Name
}

func (a *Azure) GetProviderName() string {
	return ProviderName
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
	secret, err := a.CreateOrUpdateSecret(id, config, ctx, oldSecret, false)
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
			Name:   a.Description,
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
			return "", microerror.Maskf(requestFailedError, "Failed to create application: %s", PrintOdataError(err))
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
		return "", microerror.Maskf(notFoundError, "Could not find ID of app %s", config.Name)
	}

	//Update if needed
	if needsUpdate, patch := a.computeAppUpdatePatch(config, app, parentApp); needsUpdate {
		_, err = a.Client.Applications().ByApplicationId(*id).Patch(ctx, patch, nil)
		if err != nil {
			return "", microerror.Maskf(requestFailedError, "Failed to update application: %s", PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Updated %s app %s for %s in microsoft ad tenant %s", a.Type, config.Name, a.Owner, a.TenantID))
	}
	return *id, nil
}

func (a *Azure) CreateOrUpdateSecret(id string, config provider.AppConfig, ctx context.Context, oldSecret string, skipDelete bool) (provider.ProviderSecret, error) {

	app, err := a.Client.Applications().ByApplicationId(id).Get(ctx, nil)
	if err != nil {
		return provider.ProviderSecret{}, microerror.Maskf(requestFailedError, "Failed to get application: %s", PrintOdataError(err))
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
		if skipDelete {
			a.Log.Info(fmt.Sprintf("Skipped deletion of secret %v of app %s in microsoft ad tenant %s", secret.GetKeyId(), id, a.TenantID))
		} else {
			if err = a.DeleteSecret(ctx, secret.GetKeyId(), id); err != nil {
				return provider.ProviderSecret{}, microerror.Mask(err)
			}
			a.Log.Info(fmt.Sprintf("Removed secret %v of %s app %s for %s in microsoft ad tenant %s", secret.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
		}
		needsCreation = true
	}

	// Create secret if it does not exist
	if needsCreation {
		secret, err = a.Client.Applications().ByApplicationId(id).AddPassword().Post(ctx, GetSecretCreateRequestBody(config), nil)
		if err != nil {
			return provider.ProviderSecret{}, microerror.Maskf(requestFailedError, "Failed to create secret: %s", PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Created secret %v of %s app %s for %s in microsoft ad tenant %s", secret.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
	}
	return getAzureSecret(secret, app, oldSecret)
}

func (a *Azure) DeleteSecret(ctx context.Context, secretID *uuid.UUID, appID string) error {
	requestBody := applications.NewItemRemovePasswordPostRequestBody()
	requestBody.SetKeyId(secretID)

	err := a.Client.Applications().ByApplicationId(appID).RemovePassword().Post(ctx, requestBody, nil)
	if err != nil {
		return microerror.Maskf(requestFailedError, "Failed to delete secret: %s", PrintOdataError(err))
	}
	return nil
}

func (a *Azure) DeleteApp(name string, ctx context.Context) error {
	appID, err := a.GetAppID(name)
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return microerror.Mask(err)
	}
	if err := a.Client.Applications().ByApplicationId(appID).Delete(ctx, nil); err != nil {
		return microerror.Maskf(requestFailedError, "Failed to delete application: %s", PrintOdataError(err))
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
			return microerror.Maskf(requestFailedError, "Failed to get applications: %s", PrintOdataError(err))
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

// TODO
// improve output
// include new service principal creation
func (a *Azure) GetCredentialsForAuthenticatedApp(config provider.AppConfig) (map[string]string, error) {
	ctx := context.Background()
	app, err := a.GetApp(config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return nil, microerror.Mask(err)
		}
		// Create app if it does not exist
		app = models.NewApplication()
		app.SetDisplayName(&config.Name)

		// Set permissions from parent app
		parentApp, err := a.GetApp(DexOperatorName)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		if needsUpdate, patch := computePermissionsUpdatePatch(app, parentApp); needsUpdate {
			app.SetRequiredResourceAccess(patch)
		}
		app, err = a.Client.Applications().Post(ctx, app, nil)
		if err != nil {
			return nil, microerror.Maskf(requestFailedError, "Failed to create authenticated app: %s", PrintOdataError(err))
		}
		a.Log.Info(fmt.Sprintf("Created %s app %s for %s in microsoft ad tenant %s", a.Type, config.Name, a.Owner, a.TenantID))
		if app.GetAppId() == nil {
			return nil, microerror.Maskf(notFoundError, "Could not find client ID of app %s.", config.Name)
		}
		consentURL := getAdminConsentUrl(a.TenantID, *app.GetAppId())
		a.Log.Info(fmt.Sprintf("Admin consent is needed. Please grant under the following URL: %s", consentURL))
		a.Log.Info("Please be aware that it can take a while for the app to become available. Wait a moment before logging in and granting consent.")
		err = open.Start(consentURL)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	id := app.GetId()
	if id == nil {
		return nil, microerror.Maskf(notFoundError, "Could not find ID of app %s.", config.Name)
	}
	secret, err := a.CreateOrUpdateSecret(*id, config, ctx, "", true)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	return map[string]string{
		ClientIDKey:     secret.ClientId,
		ClientSecretKey: secret.ClientSecret,
		TenantIDKey:     a.TenantID,
	}, nil
}

func (a *Azure) CleanCredentialsForAuthenticatedApp(config provider.AppConfig) error {
	app, err := a.GetApp(config.Name)
	if err != nil {
		if !IsNotFound(err) {
			return microerror.Mask(err)
		}
		return nil
	}
	id := app.GetId()
	if id == nil {
		return microerror.Maskf(notFoundError, "Could not find ID of app %s.", config.Name)
	}
	for _, c := range app.GetPasswordCredentials() {
		if credentialName := c.GetDisplayName(); credentialName != nil {
			if *credentialName == config.Name && secretChanged(c, a.clientSecret) {
				if err = a.DeleteSecret(context.Background(), c.GetKeyId(), *id); err != nil {
					return microerror.Mask(err)
				}
				a.Log.Info(fmt.Sprintf("Removed secret %v of %s app %s for %s in microsoft ad tenant %s", c.GetKeyId(), a.Type, config.Name, a.Owner, a.TenantID))
			}
		}
	}
	return nil
}

func (a *Azure) DeleteAuthenticatedApp(config provider.AppConfig) error {
	ctx := context.Background()
	installation := strings.TrimPrefix(config.Name, DexOperatorName+"-")

	// get all the dex-apps
	dexApps, err := a.Client.Applications().Get(context.Background(), GetAllAppsContainingRequestConfig("dex-app"))
	if err != nil {
		return microerror.Maskf(requestFailedError, "Failed to get dex apps: %s", PrintOdataError(err))
	}
	count := dexApps.GetOdataCount()
	if *count != 0 {
		appList := dexApps.GetValue()
		for _, app := range appList {
			if strings.Split(*app.GetDisplayName(), "-")[0] == installation {
				err := a.DeleteApp(*app.GetDisplayName(), ctx)
				if err != nil {
					return microerror.Maskf(requestFailedError, "Failed to delete dex app: %s", PrintOdataError(err))
				}
			}
		}
	}

	// get dex-operator app
	dexOperator, err := a.GetApp(config.Name)
	if err == nil {
		err := a.DeleteApp(*dexOperator.GetDisplayName(), ctx)
		if err != nil {
			return microerror.Maskf(requestFailedError, "Failed to delete dex operator app: %s", PrintOdataError(err))
		}
	} else {
		if !IsNotFound(err) {
			return microerror.Mask(err)
		}
	}
	a.Log.Info(fmt.Sprintf("Deleted all %s app resources for installation %s in microsoft ad tenant %s", a.Type, installation, a.TenantID))
	return nil
}

func (a *Azure) GetCredentialExpiry(ctx context.Context) (time.Time, error) {
	appName := key.GetDexOperatorName(a.managementClusterName)

	app, err := a.GetApp(appName)
	if err != nil {
		return time.Time{}, microerror.Mask(err)
	}

	// Find the current secret for this app
	secret, err := GetSecret(app, appName)
	if err != nil {
		return time.Time{}, microerror.Mask(err)
	}

	if endDateTime := secret.GetEndDateTime(); endDateTime != nil {
		return *endDateTime, nil
	}

	return time.Time{}, microerror.Maskf(notFoundError, "no expiry time found for Azure credential")
}
