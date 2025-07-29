package idp

import (
	"context"
	"time"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"
)

// Error definitions
var renewalError = &microerror.Error{
	Kind: "renewalError",
}

const (
	// CredentialsSecretName is the standard name for dex-operator credentials
	CredentialsSecretName = "dex-operator-credentials"
	// SelfRenewalAnnotation marks when self-renewal was performed
	SelfRenewalAnnotation = "dex-operator.giantswarm.io/last-self-renewal"
)

// CredentialsConfig represents the structure of the credentials YAML
type CredentialsConfig struct {
	Providers []ProviderConfig `yaml:",inline"`
}

// ProviderConfig represents a single provider's configuration in the credentials
type ProviderConfig struct {
	Name        string            `yaml:"name"`
	Owner       string            `yaml:"owner"`
	Credentials map[string]string `yaml:"credentials"`
	Description string            `yaml:"description,omitempty"`
}

type ProviderCredentialUpdate struct {
	ProviderName string
	Credentials  map[string]string
}

// CheckAndRotateServiceCredentials checks if any providers need credential rotation and performs it
func (s *Service) CheckAndRotateServiceCredentials(ctx context.Context) error {
	s.log.Info("Checking if dex-operator service credentials need rotation")

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

	rotationNeeded := false
	var credentialsToUpdate []ProviderCredentialUpdate

	// Check each provider for self-renewal capability
	for _, prov := range s.providers {
		// Simplified check - no type assertion needed anymore
		if !prov.SupportsServiceCredentialRenewal() {
			s.log.Info("Provider does not support service credential renewal, skipping",
				"provider", prov.GetName())
			continue
		}

		shouldRotate, err := prov.ShouldRotateServiceCredentials(ctx, selfAppConfig)
		if err != nil {
			s.log.Error(err, "Failed to check if service credentials should rotate",
				"provider", prov.GetName())
			continue
		}

		if shouldRotate {
			s.log.Info("Service credential rotation needed",
				"provider", prov.GetName())

			newCredentials, err := prov.RotateServiceCredentials(ctx, selfAppConfig)
			if err != nil {
				s.log.Error(err, "Failed to rotate service credentials",
					"provider", prov.GetName())
				continue
			}

			credentialsToUpdate = append(credentialsToUpdate, ProviderCredentialUpdate{
				ProviderName: prov.GetProviderName(),
				Credentials:  newCredentials,
			})
			rotationNeeded = true
		}
	}

	if rotationNeeded {
		s.log.Info("Updating credentials secret with rotated credentials")
		return s.updateCredentialsSecret(ctx, credentialsToUpdate)
	}

	s.log.Info("No service credential rotation needed")
	return nil
}

// updateCredentialsSecret updates the existing dex-operator-credentials secret with rotated credentials
func (s *Service) updateCredentialsSecret(ctx context.Context, updates []ProviderCredentialUpdate) error {
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

	// Parse the existing YAML credentials using proper structs
	var existingProviders []ProviderConfig
	if err := yaml.Unmarshal(credentialsData, &existingProviders); err != nil {
		return microerror.Maskf(renewalError, "Failed to parse existing credentials: %v", err)
	}

	// Update credentials for each provider
	for _, update := range updates {
		updated := false
		for i := range existingProviders {
			if existingProviders[i].Name == update.ProviderName {
				// Update with new credentials
				for key, value := range update.Credentials {
					if existingProviders[i].Credentials == nil {
						existingProviders[i].Credentials = make(map[string]string)
					}
					existingProviders[i].Credentials[key] = value
				}
				updated = true
				s.log.Info("Credentials for provider updated",
					"provider", update.ProviderName)
				break
			}
		}

		if !updated {
			return microerror.Maskf(renewalError,
				"Could not find provider %s in existing credentials", update.ProviderName)
		}
	}

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(existingProviders)
	if err != nil {
		return microerror.Maskf(renewalError, "Failed to marshal updated credentials: %v", err)
	}

	// Update the secret
	secret.Data["credentials"] = updatedData

	// Add renewal annotation using helper function
	s.addSelfRenewalAnnotation(secret)

	if err := s.Update(ctx, secret); err != nil {
		return microerror.Maskf(renewalError, "Failed to update credentials secret: %v", err)
	}

	s.log.Info("Successfully updated dex-operator-credentials secret with rotated credentials")
	return nil
}

// addSelfRenewalAnnotation is a helper function to manipulate annotations
func (s *Service) addSelfRenewalAnnotation(secret *corev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[SelfRenewalAnnotation] = time.Now().Format(time.RFC3339)
}
