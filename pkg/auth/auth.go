package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"

	"github.com/giantswarm/dex-operator/pkg/dextarget"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Config struct {
	Client                          client.Client
	Log                             logr.Logger
	Target                          dextarget.DexTarget
	ManagementClusterName           string
	ManagementClusterWriteAllGroups []string

	// Deprecated: Use Target instead. App is kept for backward compatibility.
	// If Target is nil and App is set, App will be wrapped in an AppTarget.
	App *v1alpha1.App
}

type Service struct {
	client.Client
	log                             logr.Logger
	target                          dextarget.DexTarget
	managementClusterName           string
	managementClusterWriteAllGroups []string
}

func New(c Config) (*Service, error) {
	// Backward compatibility: if Target is nil but App is set, wrap App in an AppTarget
	target := c.Target
	if target == nil && c.App != nil {
		target = dextarget.NewAppTarget(c.App)
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
	if c.ManagementClusterName == "" {
		return nil, microerror.Maskf(invalidConfigError, "no management cluster name given")
	}
	if len(c.ManagementClusterWriteAllGroups) == 0 {
		return nil, microerror.Maskf(invalidConfigError, "no write all groups given")
	}
	s := &Service{
		Client:                          c.Client,
		target:                          target,
		log:                             c.Log,
		managementClusterName:           c.ManagementClusterName,
		managementClusterWriteAllGroups: c.ManagementClusterWriteAllGroups,
	}

	return s, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	cluster := s.target.GetClusterLabel()
	nn := s.target.GetNamespacedName()

	// the auth config is only useful for workload cluster targets
	if cluster == "" || cluster == s.managementClusterName {
		return nil
	}

	apiServerPort, err := s.getAPIServerPort(cluster, nn.Namespace, ctx)
	if err != nil {
		return err
	}

	writeAllGroups, err := s.getWriteAllGroups(nn.Namespace, ctx)
	if err != nil {
		return err
	}

	config := authConfig{
		cluster:           cluster,
		name:              key.GetAuthConfigName(cluster),
		namespace:         nn.Namespace,
		managementCluster: s.managementClusterName,
		adminGroups:       writeAllGroups,
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
	cluster := s.target.GetClusterLabel()
	nn := s.target.GetNamespacedName()

	config := authConfig{
		cluster:   cluster,
		name:      key.GetAuthConfigName(cluster),
		namespace: nn.Namespace,
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

func (s *Service) getAPIServerPort(clusterID string, targetNamespace string, ctx context.Context) (int, error) {
	var namespace string
	{
		if isOrgNamespace(targetNamespace) {
			namespace = targetNamespace
		} else {
			namespace = "org-" + s.target.GetOrganizationLabel()
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

func (s *Service) getWriteAllGroups(targetNamespace string, ctx context.Context) ([]string, error) {
	writeAllGroups := s.managementClusterWriteAllGroups

	// get all additional groups that have cluster-admin role in target namespace
	roleBindings := &rbacv1.RoleBindingList{}
	if err := s.List(ctx, roleBindings, client.InNamespace(targetNamespace)); err != nil {
		return nil, err
	}
	for _, roleBinding := range roleBindings.Items {
		if roleBinding.RoleRef.Name != "cluster-admin" && roleBinding.RoleRef.Kind != "ClusterRole" {
			continue
		}
		for _, subject := range roleBinding.Subjects {
			if subject.Kind != "Group" {
				continue
			}
			if !contains(writeAllGroups, subject.Name) {
				writeAllGroups = append(writeAllGroups, subject.Name)
			}
		}
	}
	return writeAllGroups, nil
}

func isOrgNamespace(namespace string) bool {
	return strings.HasPrefix(namespace, "org-")
}

func contains(groups []string, group string) bool {
	for _, g := range groups {
		if g == group {
			return true
		}
	}
	return false
}
