package azure

import (
	"fmt"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"reflect"
	"time"

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
	Claim                 = "groups"
	Audience              = "AzureADMyOrg"
)

func ProviderScope() []string {
	return []string{"https://graph.microsoft.com/.default"}
}

func computeAppUpdatePatch(config provider.AppConfig, app models.Applicationable, parentApp models.Applicationable) (bool, models.Applicationable) {
	appPatch := models.NewApplication()
	appNeedsUpdate := false

	if needsUpdate, patch := computePermissionsUpdatePatch(app, parentApp); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetRequiredResourceAccess(patch)
	}

	if needsUpdate, patch := computeRedirectURIUpdatePatch(app, config); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetWeb(patch)
	}

	if needsUpdate, patch := computeClaimsUpdatePatch(app); needsUpdate {
		appNeedsUpdate = true
		appPatch.SetOptionalClaims(patch)
	}

	return appNeedsUpdate, appPatch
}

func computePermissionsUpdatePatch(app models.Applicationable, parentApp models.Applicationable) (bool, []models.RequiredResourceAccessable) {
	var original, patch []models.RequiredResourceAccessable
	{
		original = app.GetRequiredResourceAccess()
		patch = getPermissionCreateRequestBody(parentApp)
	}
	if reflect.DeepEqual(original, patch) {
		return false, nil
	}
	return true, patch
}

func computeRedirectURIUpdatePatch(app models.Applicationable, config provider.AppConfig) (bool, models.WebApplicationable) {
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

func computeClaimsUpdatePatch(app models.Applicationable) (bool, models.OptionalClaimsable) {
	var original, patch models.OptionalClaimsable
	{
		original = app.GetOptionalClaims()
		patch = getClaimsRequestBody()
	}
	if reflect.DeepEqual(original, patch) {
		return false, nil
	}
	return true, patch
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

func getAppCreateRequestBody(config provider.AppConfig) *models.Application {
	// Assemble request body
	app := models.NewApplication()
	app.SetDisplayName(&config.Name)
	app.SetWeb(getRedirectURIsRequestBody([]string{config.RedirectURI}))
	app.SetOptionalClaims(getClaimsRequestBody())
	app.SetIdentifierUris([]string{config.IdentifierURI})
	audience := Audience
	app.SetSignInAudience(&audience)

	return app
}

func getPermissionCreateRequestBody(parentApp models.Applicationable) []models.RequiredResourceAccessable {
	return parentApp.GetRequiredResourceAccess()
}

func getRedirectURIsRequestBody(redirectURIs []string) models.WebApplicationable {
	web := models.NewWebApplication()
	web.SetRedirectUris(redirectURIs)
	return web
}

func getClaimsRequestBody() models.OptionalClaimsable {
	claimName := Claim
	claim := models.NewOptionalClaim()
	claim.SetName(&claimName)
	claims := models.NewOptionalClaims()
	claims.SetAccessToken([]models.OptionalClaimable{claim})
	claims.SetIdToken([]models.OptionalClaimable{claim})
	claims.SetSaml2Token([]models.OptionalClaimable{claim})
	return claims
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
