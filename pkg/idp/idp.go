package idp

import (
	"context"
	"encoding/json"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	Client                      client.Client
	Log                         *logr.Logger
	App                         *v1alpha1.App
	Providers                   []provider.Provider
	ManagementClusterBaseDomain string
}

type Service struct {
	client.Client
	log                         logr.Logger
	app                         *v1alpha1.App
	providers                   []provider.Provider
	managementClusterBaseDomain string
}

func New(c Config) (*Service, error) {
	if c.App == nil {
		return nil, microerror.Maskf(invalidConfigError, "app can not be nil")
	}
	if c.Client == nil {
		return nil, microerror.Maskf(invalidConfigError, "client cannot be nil")
	}
	if c.Log == nil {
		return nil, microerror.Maskf(invalidConfigError, "log cannot be nil")
	}
	if c.Providers == nil {
		return nil, microerror.Maskf(invalidConfigError, "providers can not be nil")
	}
	if c.ManagementClusterBaseDomain == "" {
		return nil, microerror.Maskf(invalidConfigError, "no management cluster base domain given")
	}

	s := &Service{
		Client:                      c.Client,
		app:                         c.App,
		log:                         *c.Log,
		providers:                   c.Providers,
		managementClusterBaseDomain: c.ManagementClusterBaseDomain,
	}

	return s, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	// Add secret config to app instance
	dexSecretConfig := GetDexSecretConfig(s.app.Namespace)
	if !dexSecretConfigIsPresent(s.app, dexSecretConfig) {
		s.app.Spec.ExtraConfigs = append(s.app.Spec.ExtraConfigs, dexSecretConfig)
		if err := s.Update(ctx, s.app); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info("Added secret config to dex app instance.")
	}

	// Fetch secret
	secret := &corev1.Secret{}
	if err := s.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return microerror.Mask(err)
		} else {

			// Create apps for each provider and get dex config
			appConfig, err := s.GetAppConfig(ctx)
			if err != nil {
				return microerror.Mask(err)
			}
			dexConfig, err := s.CreateProviderApps(appConfig, ctx)
			if err != nil {
				return microerror.Mask(err)
			}
			data, err := json.Marshal(dexConfig)
			if err != nil {
				return microerror.Mask(err)
			}
			// Create secret
			secret = s.GetDefaultDexConfigSecret(dexSecretConfig.Name, dexSecretConfig.Namespace, data)
			if err := s.Create(ctx, secret); err != nil {
				return microerror.Mask(err)
			}
			s.log.Info("Created default dex config secret for dex app instance.")
		}
	}
	// TODO: update/rotation logic
	return nil
}

func (s *Service) ReconcileDelete(ctx context.Context) error {
	// Fetch secret if present
	dexSecretConfig := GetDexSecretConfig(s.app.Namespace)
	if dexSecretConfigIsPresent(s.app, dexSecretConfig) {
		secret := &corev1.Secret{}
		if err := s.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				return microerror.Mask(err)
			}
		} else {
			if err := s.DeleteProviderApps(key.GetIdpAppName(s.app.Namespace, s.app.Name)); err != nil {
				return microerror.Mask(err)
			}
			//delete secret if it exists
			if err := s.Delete(ctx, secret); err != nil {
				if !apierrors.IsNotFound(err) {
					return microerror.Mask(err)
				}
			} else {
				s.log.Info("Deleted default dex config secret for dex app instance.")
			}
		}
	}
	return nil
}

func (s *Service) CreateProviderApps(appConfig provider.AppConfig, ctx context.Context) (dex.DexConfig, error) {
	dexConfig := dex.DexConfig{
		Oidc: dex.DexOidc{
			Giantswarm: dex.DexOidcGiantswarm{},
		},
	}
	for _, provider := range s.providers {

		// Create the app on the identity provider
		connector, err := provider.CreateApp(appConfig, ctx)
		if err != nil {
			return dexConfig, err
		}

		// Add connector configuration to config
		dexConfig.Oidc.Giantswarm.Connectors = append(dexConfig.Oidc.Giantswarm.Connectors, connector)

	}
	return dexConfig, nil
}

func (s *Service) DeleteProviderApps(appName string) error {
	for _, provider := range s.providers {

		if err := provider.DeleteApp(appName); err != nil {
			return microerror.Mask(err)
		}
	}
	return nil
}

func (s *Service) GetAppConfig(ctx context.Context) (provider.AppConfig, error) {
	var baseDomain string
	{
		// Get the cluster values configmap if present
		if clusterValuesIsPresent(s.app) {
			clusterValuesConfigmap := &corev1.ConfigMap{}
			if err := s.Get(ctx, types.NamespacedName{
				Name:      s.app.Spec.Config.ConfigMap.Name,
				Namespace: s.app.Spec.Config.ConfigMap.Namespace},
				clusterValuesConfigmap); err != nil {
				return provider.AppConfig{}, err
			}
			// Get the base domain
			baseDomain = clusterValuesConfigmap.Data[key.BaseDomainKey]
		}
		// Vintage management cluster case
		if baseDomain == "" {
			baseDomain = s.managementClusterBaseDomain
		}
	}
	return provider.AppConfig{
		Name:        key.GetIdpAppName(s.app.Namespace, s.app.Name),
		RedirectURI: key.GetRedirectURI(baseDomain),
	}, nil
}

func (s *Service) GetDefaultDexConfigSecret(name string, namespace string, data []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				label.ManagedBy: key.DexOperatorLabelValue,
			},
		},
		Type: "Opaque",
		Data: map[string][]byte{"default": data},
	}
}
