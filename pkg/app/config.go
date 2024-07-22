package app

import (
	"context"

	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	RedirectURI          string
	Name                 string
	IssuerURI            string
	IdentifierURI        string
	SecretValidityMonths int
}

type ManagementClusterProps struct {
	Name          string
	BaseDomain    string
	IssuerAddress string
}

func GetConfig(ctx context.Context, app *v1alpha1.App, client client.Client, managementCluster ManagementClusterProps) (Config, error) {
	var baseDomain string

	// Get the cluster values configmap if present (workload cluster format)
	if ClusterValuesIsPresent(app) {
		clusterValuesConfigmap := &corev1.ConfigMap{}
		if err := client.Get(ctx, types.NamespacedName{
			Name:      app.Spec.Config.ConfigMap.Name,
			Namespace: app.Spec.Config.ConfigMap.Namespace},
			clusterValuesConfigmap); err != nil {
			return Config{}, err
		}
		// Get the base domain
		baseDomain = GetBaseDomainFromClusterValues(clusterValuesConfigmap)
	}
	issuerAddress := GetIssuerAddress(baseDomain, managementCluster.IssuerAddress, managementCluster.BaseDomain)

	return Config{
		Name:                 key.GetIdpAppName(managementCluster.Name, app.Namespace, app.Name),
		IssuerURI:            key.GetIssuerURI(issuerAddress),
		RedirectURI:          key.GetRedirectURI(issuerAddress),
		IdentifierURI:        key.GetIdentifierURI(key.GetIdpAppName(managementCluster.Name, app.Namespace, app.Name)),
		SecretValidityMonths: key.SecretValidityMonths,
	}, nil
}
