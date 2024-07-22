package key

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppLabel                       = "app.kubernetes.io/name"
	AppClusterLabel                = "giantswarm-io/cluster"
	AdminRoleName                  = "cluster-admin"
	DexAppLabelValue               = "dex-app"
	AuthConfigName                 = "default-auth-config"
	DexConfigName                  = "default-dex-config"
	DexOperatorFinalizer           = "dex-operator.finalizers.giantswarm.io/app-controller"
	DexOperatorLabelValue          = "dex-operator"
	ClusterValuesConfigmapSuffix   = "cluster-values"
	ValuesConfigMapKey             = "values"
	MCDexAppDefaultName            = "dex-app"
	MCDexAppDefaultNamespace       = "giantswarm"
	BaseDomainKey                  = "baseDomain"
	ConnectorsKey                  = "connectors"
	DexResourceURI                 = "https://dex.giantswarm.io"
	OwnerGiantswarm                = "giantswarm"
	OwnerCustomer                  = "customer"
	OwnerGiantswarmDisplayName     = "Giant Swarm"
	OwnerCustomerDisplayName       = "Customer"
	OIDCIssuerAPIServerExtraArg    = "oidc-issuer-url"
	ClusterOIDCConfigName          = "cluster-oidc-config"
	UpdateOIDCFlagsAnnotationName  = "cluster.giantswarm.io/update-oidc-flags"
	UpdateOIDCFlagsAnnotationValue = "true"
)

/*
oidc-client-id: dex-k8s-authenticator
oidc-groups-claim: groups
oidc-issuer-url: https://dex.golem.gaws.gigantic.io
oidc-username-claim: email
*/

const (
	SecretValidityMonths = 3
)

func DexLabelSelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			AppLabel: DexAppLabelValue,
		},
	}
}

func MCDexDefaultNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      MCDexAppDefaultName,
		Namespace: MCDexAppDefaultNamespace,
	}
}

func GetProviderName(owner string, name string) string {
	return fmt.Sprintf("%s-%s", owner, name)
}

func GetDexConfigName(name string) string {
	return fmt.Sprintf("%s-%s", name, DexConfigName)
}

func GetAuthConfigName(name string) string {
	return fmt.Sprintf("%s-%s", name, AuthConfigName)
}

func GetIdpAppName(installation string, namespace string, name string) string {
	return fmt.Sprintf("%s-%s-%s", installation, namespace, name)
}

func GetDefaultConnectorDescription(connectorDisplayName string, owner string) string {
	return fmt.Sprintf("%s for %s", connectorDisplayName, GetOwnerDisplayName(owner))
}

func GetOwnerDisplayName(owner string) string {
	switch owner {
	case OwnerGiantswarm:
		return OwnerGiantswarmDisplayName
	case OwnerCustomer:
		return OwnerCustomerDisplayName
	default:
		return owner
	}
}

func GetIssuerURI(issuerAddress string) string {
	return fmt.Sprintf("https://%s", issuerAddress)
}

func GetRedirectURI(issuerAddress string) string {
	return fmt.Sprintf("https://%s/callback", issuerAddress)
}

func GetIdentifierURI(name string) string {
	return fmt.Sprintf("https://dex.giantswarm.io/%s", name)
}

func GetIssuerAddress(clusterDomain string) string {
	return fmt.Sprintf("dex.%s", clusterDomain)
}

func GetVintageClusterDomain(baseDomain string) string {
	return fmt.Sprintf("g8s.%s", baseDomain)
}

func GetDexOperatorName(installation string) string {
	return fmt.Sprintf("dex-operator-%s", installation)
}

func GetClusterOIDCConfigName(appName string) string {
	return fmt.Sprintf("%s-%s", appName, ClusterOIDCConfigName)
}
