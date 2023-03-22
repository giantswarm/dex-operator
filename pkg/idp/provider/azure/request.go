package azure

import (
	"fmt"
	"reflect"
	"time"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
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
	DexOperatorName       = "dex-operator"
)

func ProviderScope() []string {
	return []string{"https://graph.microsoft.com/.default"}
}

// We compare the permissions set for the app to the permissions set on the parent app and ensure they are exactly the same
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
	if original == nil {
		return true, patch
	}
	uris := original.GetRedirectUris()
	for _, uri := range uris {
		if uri == config.RedirectURI {
			return false, nil
		}
	}
	patch.SetRedirectUris(append(uris, config.RedirectURI))
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
	if original == nil {
		return true, patch
	}
	update := false
	patch = models.NewOptionalClaims()
	{
		needsUpdate, patchClaim := computeClaimUpdatePatch(original.GetAccessToken())
		patch.SetAccessToken(patchClaim)
		update = update || needsUpdate
	}
	{
		needsUpdate, patchClaim := computeClaimUpdatePatch(original.GetIdToken())
		patch.SetIdToken(patchClaim)
		update = update || needsUpdate
	}
	{
		needsUpdate, patchClaim := computeClaimUpdatePatch(original.GetSaml2Token())
		patch.SetSaml2Token(patchClaim)
		update = update || needsUpdate
	}
	if !update {
		return false, nil
	}
	return true, patch
}

func computeClaimUpdatePatch(claims []models.OptionalClaimable) (bool, []models.OptionalClaimable) {
	for _, claim := range claims {
		if n := claim.GetName(); n != nil {
			if *n == Claim {
				return false, claims
			}
		}
	}
	return true, append(claims, getClaim())
}

func GetAppGetRequestConfig(name string) *applications.ApplicationsRequestBuilderGetRequestConfiguration {
	headers := abstractions.NewRequestHeaders()
	headers.Add("ConsistencyLevel", "eventual")
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

func getAppCreateRequestBody(config provider.AppConfig) models.Applicationable {
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
	claim := getClaim()
	claims := models.NewOptionalClaims()
	claims.SetAccessToken([]models.OptionalClaimable{claim})
	claims.SetIdToken([]models.OptionalClaimable{claim})
	claims.SetSaml2Token([]models.OptionalClaimable{claim})
	return claims
}

func getClaim() *models.OptionalClaim {
	claimName := Claim
	claim := models.NewOptionalClaim()
	claim.SetName(&claimName)
	return claim
}

func getClaimFromName(claimName string) *models.OptionalClaim {
	claim := models.NewOptionalClaim()
	claim.SetName(&claimName)
	return claim
}

func GetSecretCreateRequestBody(config provider.AppConfig) *applications.ItemAddPasswordPostRequestBody {
	keyCredential := models.NewPasswordCredential()
	keyCredential.SetDisplayName(&config.Name)

	validUntil := time.Now().AddDate(0, config.SecretValidityMonths, 0)
	keyCredential.SetEndDateTime(&validUntil)

	secret := applications.NewItemAddPasswordPostRequestBody()
	secret.SetPasswordCredential(keyCredential)

	return secret
}

func getAdminConsentUrl(organization string, clientID string) string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/adminconsent?client_id=%s", organization, clientID)
}
