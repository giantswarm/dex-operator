package idp

import (
	"context"
	"fmt"
	"time"

	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// RenewalThreshold - renew credentials 30 days before expiry
	RenewalThreshold = 30 * 24 * time.Hour
	// SelfRenewalAnnotation marks secrets as self-renewable
	SelfRenewalAnnotation = "dex-operator.giantswarm.io/self-renewal"
)

// CheckSelfRenewal checks if the operator's own credentials need renewal
// This leverages the existing AppInfo metrics and provider infrastructure
func (s *Service) CheckSelfRenewal(ctx context.Context) error {
	s.log.Info("Checking if dex-operator credentials need renewal")

	// Get the app config for the dex-operator itself (reusing existing logic)
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
	for _, provider := range s.providers {
		needsRenewal, err := s.checkProviderSelfRenewal(ctx, provider, selfAppConfig)
		if err != nil {
			s.log.Error(err, "Failed to check self-renewal", "provider", provider.GetName())
			continue
		}
		if needsRenewal {
			renewalNeeded = true
		}
	}

	if renewalNeeded {
		s.log.Info("Self-renewal needed, updating credentials")
		return s.performSelfRenewal(ctx, selfAppConfig)
	}

	s.log.Info("No self-renewal needed")
	return nil
}

// checkProviderSelfRenewal checks if a specific provider needs renewal
func (s *Service) checkProviderSelfRenewal(ctx context.Context, provider provider.Provider, appConfig provider.AppConfig) (bool, error) {
	// Check the existing AppInfo metric for expiry time
	// This reuses the existing metric infrastructure
	metricValue := AppInfo.WithLabelValues(
		s.app.Name,
		s.app.Namespace,
		provider.GetOwner(),
		provider.GetType(),
		provider.GetName(),
		appConfig.Name,
	)

	metric := &dto.Metric{}
	if err := metricValue.Write(metric); err != nil {
		// If no metric exists, assume renewal is needed
		s.log.Info("No expiry metric found, assuming renewal needed", "provider", provider.GetName())
		return true, nil
	}

	expiryTime := time.Unix(int64(metric.GetGauge().GetValue()), 0)
	timeUntilExpiry := time.Until(expiryTime)

	s.log.Info("Credential expiry check",
		"provider", provider.GetName(),
		"expiry", expiryTime,
		"time_until_expiry", timeUntilExpiry)

	return timeUntilExpiry < RenewalThreshold, nil
}

// performSelfRenewal performs the actual credential renewal
func (s *Service) performSelfRenewal(ctx context.Context, appConfig provider.AppConfig) error {
	// Create a secret to store the new credentials
	renewalSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.getSelfRenewalSecretName(),
			Namespace: s.app.Namespace,
			Labels: map[string]string{
				key.DexOperatorLabelValue: "true",
			},
			Annotations: map[string]string{
				SelfRenewalAnnotation: time.Now().Format(time.RFC3339),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: make(map[string][]byte),
	}

	// Get new credentials from each provider
	for _, provider := range s.providers {
		s.log.Info("Renewing credentials", "provider", provider.GetName())

		credentials, err := provider.GetCredentialsForAuthenticatedApp(appConfig)
		if err != nil {
			s.log.Error(err, "Failed to get new credentials", "provider", provider.GetName())
			continue
		}

		// Store credentials in the secret with provider prefix
		for key, value := range credentials {
			secretKey := fmt.Sprintf("%s_%s", provider.GetName(), key)
			renewalSecret.Data[secretKey] = []byte(value)
		}

		s.log.Info("Successfully renewed credentials", "provider", provider.GetName())
	}

	// Create or update the renewal secret
	existing := &corev1.Secret{}
	err := s.Get(ctx, types.NamespacedName{
		Name:      renewalSecret.Name,
		Namespace: renewalSecret.Namespace,
	}, existing)

	if client.IgnoreNotFound(err) != nil {
		return microerror.Mask(err)
	}

	if err != nil {
		// Create new secret
		if err := s.Create(ctx, renewalSecret); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info("Created self-renewal secret", "secret", renewalSecret.Name)
	} else {
		// Update existing secret
		existing.Data = renewalSecret.Data
		existing.Annotations[SelfRenewalAnnotation] = time.Now().Format(time.RFC3339)
		if err := s.Update(ctx, existing); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info("Updated self-renewal secret", "secret", renewalSecret.Name)
	}

	return nil
}

// getSelfRenewalSecretName returns the name for the self-renewal secret
func (s *Service) getSelfRenewalSecretName() string {
	return fmt.Sprintf("%s-renewal-credentials", s.managementClusterName)
}
