package idp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/dextarget"
	"github.com/giantswarm/dex-operator/pkg/idp/provider"
	"github.com/giantswarm/dex-operator/pkg/key"

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
	Client                         client.Client
	Log                            logr.Logger
	Target                         dextarget.DexTarget
	Providers                      []provider.Provider
	ManagementClusterBaseDomain    string
	ManagementClusterName          string
	ManagementClusterIssuerAddress string

	// Deprecated: Use Target instead. App is kept for backward compatibility.
	// If Target is nil and App is set, App will be wrapped in an AppTarget.
	App *v1alpha1.App
}

type Service struct {
	client.Client
	log                            logr.Logger
	target                         dextarget.DexTarget
	providers                      []provider.Provider
	managementClusterBaseDomain    string
	managementClusterName          string
	managementClusterIssuerAddress string
}

func New(c Config) (*Service, error) {
	// Backward compatibility: if Target is nil but App is set, wrap App in an AppTarget
	target := c.Target
	if target == nil && c.App != nil {
		target = dextarget.NewAppTarget(context.Background(), c.Client, c.App)
	}

	if target == nil {
		return nil, microerror.Maskf(invalidConfigError, "target can not be nil")
	}
	if c.Client == nil {
		return nil, microerror.Maskf(invalidConfigError, "client cannot be nil")
	}
	if (logr.Logger{}) == c.Log {
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
		Client:                         c.Client,
		target:                         target,
		log:                            c.Log,
		providers:                      c.Providers,
		managementClusterBaseDomain:    c.ManagementClusterBaseDomain,
		managementClusterName:          c.ManagementClusterName,
		managementClusterIssuerAddress: c.ManagementClusterIssuerAddress,
	}

	return s, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	// We do not handle targets that have connectors in user configs set up due to a bug where configuration in secrets can be overwritten
	//TODO: solve this gracefully
	hasUserConnectors, err := s.target.HasUserConfigWithConnectors(s.Client)
	if err != nil {
		return microerror.Mask(err)
	}
	if hasUserConnectors {
		s.log.Info(fmt.Sprintf("Dex %s has user config with connector configuration. Cancelling reconciliation. We recommend to move configuration to a managed secret.", s.target.GetTargetType()))
		return s.ReconcileDelete(ctx)
	}

	nn := s.target.GetNamespacedName()
	secretName := key.GetDexConfigName(nn.Name)

	// Add secret config to target instance if not present
	if !s.target.HasSecretConfig(secretName) {
		if err := s.target.AddSecretConfig(secretName, nn.Namespace); err != nil {
			return microerror.Mask(err)
		}
		if err := s.Update(ctx, s.target.GetObject()); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info(fmt.Sprintf("Added secret config to dex %s instance.", s.target.GetTargetType()))
	}

	// Fetch secret
	secret := &corev1.Secret{}
	if err := s.Get(ctx, types.NamespacedName{Name: secretName, Namespace: nn.Namespace}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return microerror.Mask(err)
		} else {
			secret = GetDefaultDexConfigSecret(secretName, nn.Namespace)
			if err := s.Create(ctx, secret); err != nil {
				return microerror.Mask(err)
			}
			s.log.Info(fmt.Sprintf("Created default dex config secret for dex %s instance.", s.target.GetTargetType()))
		}
	}
	{
		// Get existing connectors from the dex config secret
		oldConfig, err := getDexConfigFromSecret(secret)
		if err != nil {
			return microerror.Mask(err)
		}
		oldConnectors := getConnectorsFromConfig(oldConfig)
		// Create apps for each provider and get dex config
		appConfig, err := s.GetAppConfig(ctx)
		if err != nil {
			return microerror.Mask(err)
		}
		newConfig, err := s.CreateOrUpdateProviderApps(appConfig, ctx, oldConnectors)
		if err != nil {
			return microerror.Mask(err)
		}

		if updateSecret := s.secretDataNeedsUpdate(oldConfig, newConfig); updateSecret {
			data, err := json.Marshal(newConfig)
			if err != nil {
				return microerror.Mask(err)
			}
			// Fetching the newest version of the secret. If we are here we assume that it exists.
			if err := s.Get(ctx, types.NamespacedName{Name: secretName, Namespace: nn.Namespace}, secret); err != nil {
				return microerror.Mask(err)
			}
			secret.Data = map[string][]byte{"default": data}
			if err := s.Update(ctx, secret); err != nil {
				return microerror.Mask(err)
			}
			s.log.Info(fmt.Sprintf("Updated default dex config secret for dex %s instance.", s.target.GetTargetType()))
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
	nn := s.target.GetNamespacedName()
	secretName := key.GetDexConfigName(nn.Name)

	// Check if secret config is present
	if s.target.HasSecretConfig(secretName) {
		secret := &corev1.Secret{}
		if err := s.Get(ctx, types.NamespacedName{Name: secretName, Namespace: nn.Namespace}, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				return microerror.Mask(err)
			}
		} else {
			if err := s.DeleteProviderApps(key.GetIdpAppName(s.managementClusterName, nn.Namespace, nn.Name), ctx); err != nil {
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
				s.log.Info(fmt.Sprintf("Deleted default dex config secret for dex %s instance.", s.target.GetTargetType()))
			}
		}
		// remove dex secret config
		if err := s.target.RemoveSecretConfig(secretName, nn.Namespace); err != nil {
			return microerror.Mask(err)
		}
		if err := s.Update(ctx, s.target.GetObject()); err != nil {
			if !apierrors.IsNotFound(err) {
				return microerror.Mask(err)
			}
		} else {
			s.log.Info(fmt.Sprintf("Removed dex config secret reference from dex %s instance.", s.target.GetTargetType()))
		}
	}
	return nil
}

func (s *Service) CreateOrUpdateProviderApps(appConfig provider.AppConfig, ctx context.Context, oldConnectors map[string]dex.Connector) (dex.DexConfig, error) {
	dexConfig := dex.DexConfig{}
	customerOidcOwner := dex.DexOidcOwner{}
	giantswarmOidcOwner := dex.DexOidcOwner{}
	nn := s.target.GetNamespacedName()
	for _, provider := range s.providers {
		// Create the app on the identity provider
		providerApp, err := provider.CreateOrUpdateApp(appConfig, ctx, oldConnectors[provider.GetName()])
		if err != nil {
			return dexConfig, err
		}
		// Add connector configuration to config
		switch provider.GetOwner() {
		case key.OwnerGiantswarm:
			giantswarmOidcOwner.Connectors = append(giantswarmOidcOwner.Connectors, providerApp.Connector)
		case key.OwnerCustomer:
			customerOidcOwner.Connectors = append(customerOidcOwner.Connectors, providerApp.Connector)
		default:
			return dexConfig, microerror.Maskf(invalidConfigError, "Owner %s is not known.", provider.GetOwner())
		}
		AppInfo.WithLabelValues(nn.Name, nn.Namespace, provider.GetOwner(), provider.GetType(), provider.GetName(), appConfig.Name).Set(float64(providerApp.SecretEndDateTime.Unix()))
	}
	if len(customerOidcOwner.Connectors) > 0 {
		dexConfig.Oidc.Customer = &customerOidcOwner
	}
	if len(giantswarmOidcOwner.Connectors) > 0 {
		dexConfig.Oidc.Giantswarm = &giantswarmOidcOwner
	}
	return dexConfig, nil
}

func (s *Service) DeleteProviderApps(appName string, ctx context.Context) error {
	nn := s.target.GetNamespacedName()
	for _, provider := range s.providers {

		if err := provider.DeleteApp(appName, ctx); err != nil {
			return microerror.Mask(err)
		}
		s.log.Info(fmt.Sprintf("Deleted app %s of type %s for %s.", provider.GetName(), provider.GetType(), provider.GetOwner()))
		AppInfo.DeleteLabelValues(nn.Name, nn.Namespace, provider.GetOwner(), provider.GetType(), provider.GetName(), appName)
	}
	return nil
}

func (s *Service) secretDataNeedsUpdate(oldData dex.DexConfig, newData dex.DexConfig) bool {
	if !s.oidcOwnerNeedsUpdate(oldData.Oidc.Giantswarm, newData.Oidc.Giantswarm) && !s.oidcOwnerNeedsUpdate(oldData.Oidc.Customer, newData.Oidc.Customer) {
		oldConnectors := getConnectorsFromConfig(oldData)
		newConnectors := getConnectorsFromConfig(newData)
		return s.connectorsNeedUpdate(oldConnectors, newConnectors)
	}
	return true
}

func (s *Service) oidcOwnerNeedsUpdate(oldOwner *dex.DexOidcOwner, newOwner *dex.DexOidcOwner) bool {
	return (oldOwner != nil && newOwner == nil) ||
		(oldOwner == nil && newOwner != nil) ||
		(oldOwner != nil && newOwner != nil && len(oldOwner.Connectors) != len(newOwner.Connectors))
}

func (s *Service) connectorsNeedUpdate(oldConnectors map[string]dex.Connector, newConnectors map[string]dex.Connector) bool {
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
	nn := s.target.GetNamespacedName()

	// Get the cluster values configmap if present (workload cluster format)
	if s.target.HasClusterValuesConfig() {
		clusterValuesConfigmap := &corev1.ConfigMap{}
		cmName, cmNamespace := s.target.GetClusterValuesConfigMapRef()
		if err := s.Get(ctx, types.NamespacedName{
			Name:      cmName,
			Namespace: cmNamespace},
			clusterValuesConfigmap); err != nil {
			return provider.AppConfig{}, err
		}
		// Get the base domain
		baseDomain = getBaseDomainFromClusterValues(clusterValuesConfigmap)
	}
	issuerAddress := GetIssuerAddress(baseDomain, s.managementClusterIssuerAddress, s.managementClusterBaseDomain)

	return provider.AppConfig{
		Name:                 key.GetIdpAppName(s.managementClusterName, nn.Namespace, nn.Name),
		RedirectURI:          key.GetRedirectURI(issuerAddress),
		IdentifierURI:        key.GetIdentifierURI(key.GetIdpAppName(s.managementClusterName, nn.Namespace, nn.Name)),
		SecretValidityMonths: key.SecretValidityMonths,
	}, nil
}

func GetDefaultDexConfigSecret(name string, namespace string) *corev1.Secret {
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

func GetIssuerAddress(baseDomain string, managementClusterIssuerAddress string, managementClusterBaseDomain string) string {
	var issuerAddress string
	{
		// Derive issuer address from cluster basedomain if it exists
		if baseDomain != "" {
			issuerAddress = key.GetIssuerAddress(baseDomain)
		}

		// Otherwise fall back to management cluster issuer address if present
		if issuerAddress == "" {
			issuerAddress = managementClusterIssuerAddress
		}

		// If all else fails, fall back to the base domain (only works in vintage)
		if issuerAddress == "" {
			clusterDomain := key.GetVintageClusterDomain(managementClusterBaseDomain)
			issuerAddress = key.GetIssuerAddress(clusterDomain)
		}
	}
	return issuerAddress
}
