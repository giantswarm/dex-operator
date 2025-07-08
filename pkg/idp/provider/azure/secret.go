package azure

import (
	"strings"
	"time"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"

	"github.com/dexidp/dex/connector/microsoft"
	"github.com/giantswarm/microerror"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"gopkg.in/yaml.v2"
)

func getAzureSecret(secret models.PasswordCredentialable, app models.Applicationable, oldSecret string) (provider.ProviderSecret, error) {
	var clientSecret, clientId string
	{
		secretText := secret.GetSecretText()
		if secretText != nil && *secretText != "" {
			// This should rarely happen for existing secrets
			clientSecret = *secretText
		} else if oldSecret != "" {
			// Use old secret value (but this might be expired)
			clientSecret = oldSecret
		} else {
			return provider.ProviderSecret{}, microerror.Maskf(invalidConfigError, "Cannot retrieve secret value for existing secret and no old secret provided")
		}
		if app.GetAppId() == nil || *app.GetAppId() == "" {
			return provider.ProviderSecret{}, microerror.Maskf(notFoundError, "Could not find client ID for secret")
		}
		clientId = *app.GetAppId()
	}
	var endDateTime *time.Time
	{
		if endDateTime = secret.GetEndDateTime(); endDateTime == nil {
			return provider.ProviderSecret{}, microerror.Maskf(notFoundError, "Could not find expiry time for secret")
		}
	}
	return provider.ProviderSecret{
		ClientSecret: clientSecret,
		ClientId:     clientId,
		EndDateTime:  *endDateTime,
	}, nil
}

func secretExpired(secret models.PasswordCredentialable) bool {
	bestBefore := secret.GetEndDateTime()
	if bestBefore == nil {
		return true
	}
	if bestBefore.Before(time.Now().Add(10 * 24 * time.Hour)) {
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

func GetSecret(app models.Applicationable, name string) (models.PasswordCredentialable, error) {
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
