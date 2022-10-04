package idp

import (
	"context"
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
	Client    client.Client
	Log       *logr.Logger
	App       *v1alpha1.App
	Providers []provider.Provider
}

type Service struct {
	client.Client
	log       logr.Logger
	app       *v1alpha1.App
	providers []provider.Provider
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

	s := &Service{
		Client:    c.Client,
		app:       c.App,
		log:       *c.Log,
		providers: c.Providers,
	}

	return s, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	// Add secret config to app instance
	dexSecretConfig := getDexSecretConfig(s.app)
	if !dexSecretConfigIsPresent(s.app, dexSecretConfig) {
		s.app.Spec.ExtraConfigs = append(s.app.Spec.ExtraConfigs, dexSecretConfig)
		if err := s.Update(ctx, s.app); err != nil {
			return err
		}
		s.log.Info("Added secret config to dex app instance.")
	}

	// Fetch secret
	secret := &corev1.Secret{}
	if err := s.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// Create Secret if not found
		for _, provider := range s.providers {
			provider.CreateApp()
		}
		secret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dexSecretConfig.Name,
				Namespace: dexSecretConfig.Namespace,
				Labels: map[string]string{
					label.ManagedBy: key.DexOperatorLabelValue,
				},
			},
		}
		if err := s.Create(ctx, secret); err != nil {
			return err
		}
		s.log.Info("Created default dex config secret for dex app instance.")
	}
	// TODO: update/rotation logic
	return nil
}

func (s *Service) ReconcileDelete(ctx context.Context) error {
	// Fetch secret if present
	dexSecretConfig := getDexSecretConfig(s.app)
	if dexSecretConfigIsPresent(s.app, dexSecretConfig) {
		secret := &corev1.Secret{}
		if err := s.Get(ctx, types.NamespacedName{Name: dexSecretConfig.Name, Namespace: dexSecretConfig.Namespace}, secret); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		} else {
			// TODO: idp logic
			//delete secret if it exists
			if err := s.Delete(ctx, secret); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			} else {
				s.log.Info("Deleted default dex config secret for dex app instance.")
			}
		}
	}
	return nil
}
