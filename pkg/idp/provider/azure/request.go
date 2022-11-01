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

func getPermissionCreateRequestBody(parentApp models.Applicationable) models.Applicationable {
	app := models.NewApplication()
	app.SetRequiredResourceAccess(parentApp.GetRequiredResourceAccess())
	return app
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
	// Redirect URIs
	web := models.NewWebApplication()
	web.SetRedirectUris([]string{config.RedirectURI})

	// Claims
	claimName := Claim
	claim := models.NewOptionalClaim()
	claim.SetName(&claimName)
	claims := models.NewOptionalClaims()
	claims.SetAccessToken([]models.OptionalClaimable{claim})
	claims.SetIdToken([]models.OptionalClaimable{claim})
	claims.SetSaml2Token([]models.OptionalClaimable{claim})

	// Audience
	audience := Audience

	// Assemble request body
	app := models.NewApplication()
	app.SetDisplayName(&config.Name)
	app.SetWeb(web)
	app.SetOptionalClaims(claims)
	app.SetIdentifierUris([]string{config.IdentifierURI})
	app.SetSignInAudience(&audience)

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
