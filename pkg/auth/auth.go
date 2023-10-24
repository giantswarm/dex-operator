package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Config struct {
	Client                client.Client
	Log                   *logr.Logger
	App                   *v1alpha1.App
	ManagementClusterName string
	WriteAllGroups        []string
}

type Service struct {
	client.Client
	log                   logr.Logger
	app                   *v1alpha1.App
	managementClusterName string
	writeAllGroups        []string
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
	if c.ManagementClusterName == "" {
		return nil, microerror.Maskf(invalidConfigError, "no management cluster name given")
	}
	if len(c.WriteAllGroups) == 0 {
		return nil, microerror.Maskf(invalidConfigError, "no write all groups given")
	}
	s := &Service{
		Client:                c.Client,
		app:                   c.App,
		log:                   *c.Log,
		managementClusterName: c.ManagementClusterName,
		writeAllGroups:        c.WriteAllGroups,
	}

	return s, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	cluster := s.app.GetLabels()[label.Cluster]
	// the auth config is only useful for workload cluster apps
	if cluster == "" || cluster == s.managementClusterName {
		return nil
	}

	apiServerPort, err := s.getAPIServerPort(cluster, ctx)
	if err != nil {
		return err
	}

	config := authConfig{
		cluster:           cluster,
		name:              key.GetAuthConfigName(cluster),
		namespace:         s.app.Namespace,
		managementCluster: s.managementClusterName,
		adminGroups:       s.writeAllGroups,
		apiServerPort:     apiServerPort,
	}

	// fetch auth config
	desired, err := getAuthConfigMap(config)
	if err != nil {
		return err
	}
	current := &corev1.ConfigMap{}
	if err := s.Get(ctx, types.NamespacedName{
		Name:      config.name,
		Namespace: config.namespace},
		current); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		} else {
			// create auth config
			if err := s.Create(ctx, desired); err != nil {
				return err
			}
			s.log.Info(fmt.Sprintf("Created auth configmap %s/%s.", config.namespace, config.name))
			current = desired
		}
	}
	// update auth config
	if current.Data[key.ValuesConfigMapKey] != desired.Data[key.ValuesConfigMapKey] {
		current.Data[key.ValuesConfigMapKey] = desired.Data[key.ValuesConfigMapKey]
		if err := s.Update(ctx, current); err != nil {
			return err
		}
		s.log.Info(fmt.Sprintf("Updated auth configmap %s/%s.", config.namespace, config.name))
	}
	// add finalizer
	if !controllerutil.ContainsFinalizer(current, key.DexOperatorFinalizer) {
		controllerutil.AddFinalizer(current, key.DexOperatorFinalizer)
		if err := s.Update(ctx, current); err != nil {
			return err
		}
		s.log.Info(fmt.Sprintf("Added finalizer to auth configmap %s/%s.", config.namespace, config.name))
	}
	return nil
}

func (s *Service) ReconcileDelete(ctx context.Context) error {
	cluster := s.app.GetLabels()[label.Cluster]
	config := authConfig{
		cluster:   cluster,
		name:      key.GetAuthConfigName(cluster),
		namespace: s.app.Namespace,
	}

	cm := &corev1.ConfigMap{}
	if err := s.Get(ctx, types.NamespacedName{
		Name:      config.name,
		Namespace: config.namespace},
		cm); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		} else {
			return nil
		}
	}
	// remove finalizer
	if controllerutil.ContainsFinalizer(cm, key.DexOperatorFinalizer) {
		controllerutil.RemoveFinalizer(cm, key.DexOperatorFinalizer)
		if err := s.Update(ctx, cm); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			} else {
				return nil
			}
		}
		s.log.Info(fmt.Sprintf("Removed finalizer from auth configmap %s/%s.", config.namespace, config.name))
	}
	// Delete cm
	if err := s.Delete(ctx, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		} else {
			return nil
		}
	}
	s.log.Info(fmt.Sprintf("Deleted auth configmap %s/%s.", config.namespace, config.name))
	return nil
}

func (s *Service) getAPIServerPort(clusterID string, ctx context.Context) (int, error) {
	var namespace string
	{
		if isOrgNamespace(s.app.Namespace) {
			namespace = s.app.Namespace
		} else {
			namespace = "org-" + s.app.GetLabels()[label.Organization]
		}
	}
	cluster := &capi.Cluster{}
	if err := s.Get(ctx, types.NamespacedName{
		Name:      clusterID,
		Namespace: namespace},
		cluster); err != nil {
		return 0, err
	}
	return int(cluster.Spec.ControlPlaneEndpoint.Port), nil
}

func isOrgNamespace(namespace string) bool {
	return strings.HasPrefix(namespace, "org-")
}
