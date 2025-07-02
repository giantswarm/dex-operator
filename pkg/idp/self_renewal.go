package idp

import (
	"context"
	"time"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/idp/provider/azure"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Error definitions
var renewalError = &microerror.Error{
	Kind: "renewalError",
}

const (
	// RenewalThreshold - renew credentials 30 days before expiry
	RenewalThreshold = 30 * 24 * time.Hour
	// CredentialsSecretName is the standard name for dex-operator credentials
	CredentialsSecretName = "dex-operator-credentials"
	// SelfRenewalAnnotation marks when self-renewal was performed
	SelfRenewalAnnotation = "dex-operator.giantswarm.io/last-self-renewal"
)

// CheckSelfRenewal checks if the operator's own Azure credentials need renewal
func (s *Service) CheckSelfRenewal(ctx context.Context) error {
	s.log.Info("Checking if dex-operator credentials need renewal")

	// Get the app config for the dex-operator itself
	appConfig, err := s.GetAppConfig(ctx)
	if err != nil {
		return microerror.Mask(err)
	}

	// Use the operator's own name instead of a dex app name
	selfAppConfig := provider.AppConfig{
		Name:                 key.GetDexOperatorName(s.managementClusterName),
		RedirectURI:          appConfig.RedirectURI,
		IdentifierURI:        key.GetIdentifierURI(key.GetDexOperatorName(s.managementClusterName)),
		SecretValidityMonths: key.SecretValidityMonths,
	}

	renewalNeeded := false
	var azureProvider provider.Provider

	// Only check Azure providers for renewal
	for _, prov := range s.providers {
		if prov.GetProviderName() == azure.ProviderName {
			needsRenewal, err := s.checkAzureProviderSelfRenewal(ctx, prov, selfAppConfig)
			if err != nil {
				s.log.Error(err, "Failed to check Azure self-renewal", "provider", prov.GetName())
				continue
			}
			if needsRenewal {
				renewalNeeded = true
				azureProvider = prov
			}
			break // Only one Azure provider expected
		}
	}

	if renewalNeeded && azureProvider != nil {
		s.log.Info("Self-renewal needed for Azure credentials, updating...")
		return s.performAzureSelfRenewal(ctx, azureProvider, selfAppConfig)
	}

	s.log.Info("No Azure self-renewal needed")
	return nil
}

// checkAzureProviderSelfRenewal checks if Azure provider needs renewal
func (s *Service) checkAzureProviderSelfRenewal(ctx context.Context, prov provider.Provider, appConfig provider.AppConfig) (bool, error) {
	// Cast to Azure provider to access GetCredentialExpiry method
	azureProvider, ok := prov.(*azure.Azure)
	if !ok {
		s.log.Info("Provider is not Azure type, skipping renewal check", "provider", prov.GetName())
		return false, nil
	}

	expiryTime, err := azureProvider.GetCredentialExpiry(ctx)
	if err != nil {
		s.log.Info("Could not get Azure credential expiry, assuming renewal needed",
			"provider", prov.GetName(), "error", err)
		return true, nil
	}

	timeUntilExpiry := time.Until(expiryTime)
	s.log.Info("Azure credential expiry check",
		"provider", prov.GetName(),
		"expiry", expiryTime,
		"time_until_expiry", timeUntilExpiry)

	return timeUntilExpiry < RenewalThreshold, nil
}

// performAzureSelfRenewal performs the actual Azure credential renewal
func (s *Service) performAzureSelfRenewal(ctx context.Context, azureProvider provider.Provider, appConfig provider.AppConfig) error {
	s.log.Info("Renewing Azure credentials", "provider", azureProvider.GetName())

	// Get new credentials from Azure
	credentials, err := azureProvider.GetCredentialsForAuthenticatedApp(appConfig)
	if err != nil {
		return microerror.Maskf(renewalError, "Failed to get new Azure credentials: %v", err)
	}

	s.log.Info("Successfully renewed Azure credentials", "provider", azureProvider.GetName())

	// Update the existing dex-operator-credentials secret
	return s.updateCredentialsSecret(ctx, azureProvider, credentials)
}

// updateCredentialsSecret updates the existing dex-operator-credentials secret with new Azure credentials
func (s *Service) updateCredentialsSecret(ctx context.Context, azureProvider provider.Provider, newCredentials map[string]string) error {
	// Get the existing credentials secret
	secret := &corev1.Secret{}
	err := s.Get(ctx, types.NamespacedName{
		Name:      CredentialsSecretName,
		Namespace: s.app.Namespace,
	}, secret)
	if err != nil {
		return microerror.Maskf(renewalError, "Failed to get existing credentials secret: %v", err)
	}

	// Decode the existing credentials
	credentialsData, exists := secret.Data["credentials"]
	if !exists {
		return microerror.Maskf(renewalError, "No credentials data found in secret")
	}

	// Parse the existing YAML credentials
	var existingProviders []map[string]interface{}
	if err := yaml.Unmarshal(credentialsData, &existingProviders); err != nil {
		return microerror.Maskf(renewalError, "Failed to parse existing credentials: %v", err)
	}

	// Update the Azure provider credentials
	updated := false
	for _, providerConfig := range existingProviders {
		if name, ok := providerConfig["name"].(string); ok && name == azure.ProviderName {
			// Update the Azure credentials
			if credsMap, ok := providerConfig["credentials"].(map[interface{}]interface{}); ok {
				// Update with new credentials
				for key, value := range newCredentials {
					credsMap[key] = value
				}
				updated = true
				s.log.Info("Updated Azure credentials in existing secret")
				break
			}
		}
	}

	if !updated {
		return microerror.Maskf(renewalError, "Could not find Azure provider in existing credentials")
	}

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(existingProviders)
	if err != nil {
		return microerror.Maskf(renewalError, "Failed to marshal updated credentials: %v", err)
	}

	// Update the secret
	secret.Data["credentials"] = updatedData

	// Add renewal annotation
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[SelfRenewalAnnotation] = time.Now().Format(time.RFC3339)

	if err := s.Update(ctx, secret); err != nil {
		return microerror.Maskf(renewalError, "Failed to update credentials secret: %v", err)
	}

	s.log.Info("Successfully updated dex-operator-credentials secret with new Azure credentials")
	return nil
}
