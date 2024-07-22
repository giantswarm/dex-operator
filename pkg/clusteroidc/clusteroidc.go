package clusteroidc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/giantswarm/dex-operator/pkg/app"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Config struct {
	Client                         client.Client
	Log                            logr.Logger
	App                            *v1alpha1.App
	ManagementClusterBaseDomain    string
	ManagementClusterName          string
	ManagementClusterIssuerAddress string
}

type Service struct {
	client.Client
	log               logr.Logger
	app               *v1alpha1.App
	managementCluster app.ManagementClusterProps
}

func New(c Config) (*Service, error) {
	return &Service{
		Client: c.Client,
		log:    c.Log,
		app:    c.App,
		managementCluster: app.ManagementClusterProps{
			Name:          c.ManagementClusterName,
			BaseDomain:    c.ManagementClusterBaseDomain,
			IssuerAddress: c.ManagementClusterIssuerAddress,
		},
	}, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	// Check if the "Update OIDC Flags" annotation is present
	if s.app.Annotations[key.UpdateOIDCFlagsAnnotationName] != key.UpdateOIDCFlagsAnnotationValue {
		// Annotation is not present, end reconciliation
		return nil
	}

	s.log.Info(fmt.Sprintf("Detected cluster.giantswarm.io/update-oidc-flags annotation in Dex app %s/%s, will apply OIDC flags", s.app.Name, s.app.Namespace))

	// Read "giantswarm-io/cluster" label from the app CR
	clusterName, ok := s.app.Labels[key.AppClusterLabel]
	if !ok {
		// End reconciliation if label not found
		s.log.Info(fmt.Sprintf("Dex app %s/%s does not have the giantswarm-io/cluster label, unable to determine workload cluster", s.app.Namespace, s.app.Name))
		return s.removeAnnotationAndUpdateApp(ctx)
	}

	appConfig, err := app.GetConfig(ctx, s.app, s.Client, s.managementCluster)
	if err != nil {
		return microerror.Mask(err)
	}

	// Check if OIDC flags already exist in the cluster
	oidcFlagsExist, err := s.oidcFlagsPresentInCLuster(ctx, clusterName)
	if err != nil {
		if IsOIDCFlagsConfigNotFound(err) {
			// OIDC flags cannot be checked, end reconciliation
			s.log.Info(fmt.Sprintf("Unable to check if OIDC flags are present in the %s cluster: %v", clusterName, microerror.Cause(err)))
			return s.removeAnnotationAndUpdateApp(ctx)
		}
		return microerror.Mask(err)
	}

	// OIDC flags already exist in the cluster, end reconciliation
	if oidcFlagsExist {
		s.log.Info(fmt.Sprintf("OIDC flags are already configured in the %s cluster, skipping", clusterName))
		return s.removeAnnotationAndUpdateApp(ctx)
	}

	// If not, add them to the config OR create a new extra config and reference it in the extra config of the app
	err = s.createOrUpdateOIDCConfigMap(ctx, clusterName, appConfig.IssuerURI)
	if err != nil {
		return microerror.Mask(err)
	}

	// Update extra config if needed
	if !oidcExtraConfigPresent(s.app) {
		oidcExtraConfig := GetOIDCFlagsExtraConfig(s.app)
		s.app.Spec.ExtraConfigs = append(s.app.Spec.ExtraConfigs, oidcExtraConfig)
	}

	// Remove the "Update OIDC Flags annotation"
	return s.removeAnnotationAndUpdateApp(ctx)
}

func (s *Service) ReconcileDelete(ctx context.Context) error {
	if oidcExtraConfigPresent(s.app) {
		oidcFlagsExtraConfig := GetOIDCFlagsExtraConfig(s.app)
		s.app.Spec.ExtraConfigs = app.RemoveExtraConfig(s.app.Spec.ExtraConfigs, oidcFlagsExtraConfig)
		err := s.Update(ctx, s.app)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	configMap := &v1.ConfigMap{}
	if err := s.Get(ctx, types.NamespacedName{Name: key.GetClusterOIDCConfigName(s.app.Name), Namespace: s.app.Namespace}, configMap); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return microerror.Mask(err)
	}

	if controllerutil.ContainsFinalizer(configMap, key.DexOperatorFinalizer) {
		controllerutil.RemoveFinalizer(configMap, key.DexOperatorFinalizer)
		if err := s.Update(ctx, configMap); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return microerror.Mask(err)
		} else {
			s.log.Info(fmt.Sprintf("Removed finalizer from OIDC flags configmap %s/%s", configMap.Namespace, configMap.Name))
		}
	}
	err := s.Delete(ctx, configMap)
	if err != nil && !apierrors.IsNotFound(err) {
		return microerror.Mask(err)
	}

	return nil
}

// Read configuration of the cluster from the label
// User KubeadmControlPlane as a single source of truth
func (s *Service) oidcFlagsPresentInCLuster(ctx context.Context, clusterName string) (bool, error) {
	kubeadmControlPlane := &v1beta1.KubeadmControlPlane{}
	err := s.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: s.app.Namespace}, kubeadmControlPlane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, microerror.Maskf(oidcFlagsConfigNotFoundError, "resource not found: %v", err)
		}
		return false, microerror.Mask(err)
	}

	if kubeadmControlPlane.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
		return false, oidcFlagsConfigNotFoundError
	}

	_, issuerUrlExists := kubeadmControlPlane.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs[key.OIDCIssuerAPIServerExtraArg]
	return issuerUrlExists, nil
}

func (s *Service) createOrUpdateOIDCConfigMap(ctx context.Context, clusterName, clusterIssuer string) error {
	configMap := &v1.ConfigMap{}
	configMapName := key.GetClusterOIDCConfigName(s.app.Name)
	if err := s.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: s.app.Namespace}, configMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return microerror.Mask(err)
		} else {
			configMap = GetOIDCFlagsConfigMap(configMapName, s.app.Namespace)
			if err = s.Create(ctx, configMap); err != nil {
				return microerror.Mask(err)
			}
		}
	}

	desiredData, err := CreateOIDCFlagsConfigMapValues(clusterIssuer)
	if err != nil {
		return microerror.Mask(err)
	}

	needsUpdate := false
	if !reflect.DeepEqual(desiredData, configMap.BinaryData) {
		configMap.BinaryData = desiredData
		needsUpdate = true
	}

	if !controllerutil.ContainsFinalizer(configMap, key.DexOperatorFinalizer) {
		controllerutil.AddFinalizer(configMap, key.DexOperatorFinalizer)
		needsUpdate = true
	}

	if needsUpdate {
		err = s.Update(ctx, configMap)
		s.log.Info(fmt.Sprintf("Updated the configuration of OIDC flags in the %s cluster", clusterName))
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

func (s *Service) removeAnnotationAndUpdateApp(ctx context.Context) error {
	delete(s.app.Annotations, key.UpdateOIDCFlagsAnnotationName)
	err := s.Update(ctx, s.app)
	if err != nil {
		return microerror.Mask(err)
	}
	s.log.Info("Updated the Dex app and removed the cluster.giantswarm.io/update-oidc-flags annotation")
	return nil
}

func GetOIDCFlagsConfigMap(name, namespace string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				label.ManagedBy: key.DexOperatorLabelValue,
			},
		},
		BinaryData: map[string][]byte{},
	}
}

func CreateOIDCFlagsConfigMapValues(clusterIssuer string) (map[string][]byte, error) {
	values := map[string]interface{}{
		"global": map[string]interface{}{
			"controlPlane": map[string]interface{}{
				"oidc": map[string]interface{}{
					"issuerUrl":     clusterIssuer,
					"clientId":      "dex-k8s-authenticator",
					"usernameClaim": "email",
					"groupsClaim":   "groups",
				},
			},
		},
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{key.ValuesConfigMapKey: data}, nil
}
