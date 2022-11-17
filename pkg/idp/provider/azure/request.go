package azure

import (
	"fmt"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
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

func getPermissionCreateRequestBody(parentApp models.Applicationable) []models.RequiredResourceAccessable {
	return parentApp.GetRequiredResourceAccess()
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

func getRedirectURIsRequestBody(redirectURIs []string) models.WebApplicationable {
	web := models.NewWebApplication()
	web.SetRedirectUris(redirectURIs)
	return web
}

func getClaimsRequestBody() *models.OptionalClaims {
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
