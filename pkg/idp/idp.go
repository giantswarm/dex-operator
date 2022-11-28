package idp

import (
	"context"
	"encoding/json"
	"fmt"
	"giantswarm/dex-operator/pkg/dex"
	"giantswarm/dex-operator/pkg/idp/provider"
	"giantswarm/dex-operator/pkg/key"
	"reflect"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Config struct {
	Client                      client.Client
	Log                         *logr.Logger
	App                         *v1alpha1.App
	Providers                   []provider.Provider
	ManagementClusterBaseDomain string
	ManagementClusterName       string
}

type Service struct {
	client.Client
	log                         logr.Logger
	app                         *v1alpha1.App
	providers                   []provider.Provider
	managementClusterBaseDomain string
	managementClusterName       string
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
	if c.ManagementClusterName == "" {
		return nil, microerror.Maskf(invalidConfigError, "no management cluster name given")
	}
	s := &Service{
		Client:                      c.Client,
		app:                         c.App,
		log:                         *c.Log,
		providers:                   c.Providers,
		managementClusterBaseDomain: c.ManagementClusterBaseDomain,
		managementClusterName:       c.ManagementClusterName,
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
			secret = s.GetDefaultDexConfigSecret(dexSecretConfig.Name, dexSecretConfig.Namespace)
			if err := s.Create(ctx, secret); err != nil {
				return microerror.Mask(err)
			}
			s.log.Info("Applied default dex config secret for dex app instance.")
		}
	}
	{
		// Get existing connectors from the dex config secret
		oldConnectors, err := getConnectorsFromSecret(secret)
		if err != nil {
			return microerror.Mask(err)
		}
		// Create apps for each provider and get dex config
		appConfig, err := s.GetAppConfig(ctx)
		if err != nil {
			return microerror.Mask(err)
		}
		dexConfig, err := s.CreateOrUpdateProviderApps(appConfig, ctx, oldConnectors)
		if err != nil {
			return microerror.Mask(err)
		}
		data, err := json.Marshal(dexConfig)
		if err != nil {
			return microerror.Mask(err)
		}
		// Fetching the newest version of the secret. If we are here we assume that it exists.
		if err := s.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
			return microerror.Mask(err)
		}
		secret.Data = map[string][]byte{"default": data}
		// Get new connectors from the dex config secret
		newConnectors, err := getConnectorsFromSecret(secret)
		if err != nil {
			return microerror.Mask(err)
		}
		if updateSecret := s.secretNeedsUpdate(oldConnectors, newConnectors); updateSecret {
			if err := s.Update(ctx, secret); err != nil {
				return microerror.Mask(err)
			}
			s.log.Info("Applied default dex config secret for dex app instance.")
		}
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(secret, key.DexOperatorFinalizer) {
		controllerutil.AddFinalizer(secret, key.DexOperatorFinalizer)
		if err := s.Update(ctx, secret); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info("Added finalizer to default dex config secret.")
	}
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
			if err := s.DeleteProviderApps(key.GetIdpAppName(s.managementClusterName, s.app.Namespace, s.app.Name), ctx); err != nil {
				return microerror.Mask(err)
			}
			// remove finalizer
			if controllerutil.ContainsFinalizer(secret, key.DexOperatorFinalizer) {
				controllerutil.RemoveFinalizer(secret, key.DexOperatorFinalizer)
				if err := s.Update(ctx, secret); err != nil {
					if !apierrors.IsNotFound(err) {
						return microerror.Mask(err)
					}
				} else {
					s.log.Info("Removed finalizer from default dex config secret.")
				}
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

func (s *Service) CreateOrUpdateProviderApps(appConfig provider.AppConfig, ctx context.Context, oldConnectors map[string]dex.Connector) (dex.DexConfig, error) {
	dexConfig := dex.DexConfig{
		Oidc: dex.DexOidc{
			Giantswarm: dex.DexOidcOwner{},
			Customer:   dex.DexOidcOwner{},
		},
	}
	for _, provider := range s.providers {
		// Create the app on the identity provider
		connector, err := provider.CreateOrUpdateApp(appConfig, ctx, oldConnectors[provider.GetName()])
		if err != nil {
			return dexConfig, err
		}
		// Add connector configuration to config
		switch provider.GetOwner() {
		case key.OwnerGiantswarm:
			dexConfig.Oidc.Giantswarm.Connectors = append(dexConfig.Oidc.Giantswarm.Connectors, connector)
		case key.OwnerCustomer:
			dexConfig.Oidc.Customer.Connectors = append(dexConfig.Oidc.Customer.Connectors, connector)
		default:
			return dexConfig, microerror.Maskf(invalidConfigError, "Owner %s is not known.", provider.GetOwner())
		}
	}
	return dexConfig, nil
}

func (s *Service) DeleteProviderApps(appName string, ctx context.Context) error {
	for _, provider := range s.providers {

		if err := provider.DeleteApp(appName, ctx); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info(fmt.Sprintf("Deleted app %s of type %s for %s.", provider.GetName(), provider.GetType(), provider.GetOwner()))
	}
	return nil
}

func (s *Service) secretNeedsUpdate(oldConnectors map[string]dex.Connector, newConnectors map[string]dex.Connector) bool {
	needsUpdate := false
	for provider, connector := range newConnectors {
		oldConnector, exists := oldConnectors[provider]
		// connector is newly added
		if !exists {
			needsUpdate = true
			s.log.Info(fmt.Sprintf("Created app %s of type %s.", connector.Name, connector.Type))
		} else {
			// connector has changed
			if !reflect.DeepEqual(oldConnector, connector) {
				needsUpdate = true
				s.log.Info(fmt.Sprintf("Updated app %s of type %s.", connector.Name, connector.Type))
			}
		}
	}
	for provider, connector := range oldConnectors {
		_, exists := newConnectors[provider]
		// connector was removed
		if !exists {
			needsUpdate = true
			s.log.Info(fmt.Sprintf("App %s of type %s was removed. Please check provider for possible leftovers", connector.Name, connector.Type))
		}
	}
	return needsUpdate
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
			baseDomain = getBaseDomainFromClusterValues(clusterValuesConfigmap)
		}
		// Vintage management cluster case
		if baseDomain == "" {
			baseDomain = s.managementClusterBaseDomain
		}
	}
	return provider.AppConfig{
		Name:          key.GetIdpAppName(s.managementClusterName, s.app.Namespace, s.app.Name),
		RedirectURI:   key.GetRedirectURI(baseDomain),
		IdentifierURI: key.GetIdentifierURI(key.GetIdpAppName(s.managementClusterName, s.app.Namespace, s.app.Name)),
	}, nil
}

func (s *Service) GetDefaultDexConfigSecret(name string, namespace string) *corev1.Secret {
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
		Data: map[string][]byte{},
	}
}
