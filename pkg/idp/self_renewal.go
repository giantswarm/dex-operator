package idp

import (
	"context"
	"time"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
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
	// RestartAnnotation triggers pod restarts
	RestartAnnotation = "dex-operator.giantswarm.io/restarted-at"
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
		if err := s.updateCredentialsSecret(ctx, credentialsToUpdate); err != nil {
			return microerror.Mask(err)
		}

		// Restart pods after successful credential rotation
		s.log.Info("Triggering pod restarts after credential rotation")
		if err := s.restartRelatedPods(ctx); err != nil {
			// Log the error but don't fail the whole operation
			s.log.Error(err, "Failed to restart some pods after credential rotation")
		}
	} else {
		s.log.Info("No service credential rotation needed")
	}

	return nil
}

// restartRelatedPods triggers rolling restarts of dex-app and dex-operator deployments
func (s *Service) restartRelatedPods(ctx context.Context) error {
	restartTimestamp := time.Now().Format(time.RFC3339)

	// Restart dex-app deployment
	if err := s.restartDeployment(ctx, "dex-app", s.app.Namespace, restartTimestamp); err != nil {
		s.log.Error(err, "Failed to restart dex-app deployment")
		// Continue to try restarting other deployments
	}

	// Also check for dex-app in giantswarm namespace (management cluster)
	if err := s.restartDeployment(ctx, "dex-app", "giantswarm", restartTimestamp); err != nil {
		// This might not exist, so just log at debug level
		s.log.V(1).Info("Could not restart dex-app in giantswarm namespace", "error", err)
	}

	// Restart dex-operator deployment
	if err := s.restartDeployment(ctx, "dex-operator", s.app.Namespace, restartTimestamp); err != nil {
		s.log.Error(err, "Failed to restart dex-operator deployment")
	}

	// Also restart dex-operator in giantswarm namespace if different
	if s.app.Namespace != "giantswarm" {
		if err := s.restartDeployment(ctx, "dex-operator", "giantswarm", restartTimestamp); err != nil {
			s.log.V(1).Info("Could not restart dex-operator in giantswarm namespace", "error", err)
		}
	}

	return nil
}

// restartDeployment adds an annotation to trigger a rolling restart of a deployment
func (s *Service) restartDeployment(ctx context.Context, name, namespace, timestamp string) error {
	deployment := &appsv1.Deployment{}
	err := s.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, deployment)

	if err != nil {
		return microerror.Maskf(renewalError, "Failed to get deployment %s/%s: %v", namespace, name, err)
	}

	// Add or update the restart annotation on the pod template
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}

	// Check if we need to update
	currentTimestamp := deployment.Spec.Template.Annotations[RestartAnnotation]
	if currentTimestamp == timestamp {
		s.log.Info("Deployment already restarted with current timestamp",
			"deployment", name,
			"namespace", namespace,
			"timestamp", timestamp)
		return nil
	}

	deployment.Spec.Template.Annotations[RestartAnnotation] = timestamp

	if err := s.Update(ctx, deployment); err != nil {
		return microerror.Maskf(renewalError, "Failed to update deployment %s/%s: %v", namespace, name, err)
	}

	s.log.Info("Triggered deployment restart after credential rotation",
		"deployment", name,
		"namespace", namespace,
		"timestamp", timestamp)

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
