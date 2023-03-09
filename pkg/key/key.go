package key

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AppLabel                     = "app.kubernetes.io/name"
	DexAppLabelValue             = "dex-app"
	DexConfigName                = "default-dex-config"
	DexOperatorFinalizer         = "dex-operator.finalizers.giantswarm.io/app-controller"
	DexOperatorLabelValue        = "dex-operator"
	ClusterValuesConfigmapSuffix = "cluster-values"
	ClusterValuesConfigMapKey    = "values"
	BaseDomainKey                = "baseDomain"
	DexResourceURI               = "https://dex.giantswarm.io"
	OwnerGiantswarm              = "giantswarm"
	OwnerCustomer                = "customer"
)

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
func GetProviderName(owner string, name string) string {
	return fmt.Sprintf("%s-%s", owner, name)
}

func GetIdpAppName(installation string, namespace string, name string) string {
	return fmt.Sprintf("%s-%s-%s", installation, namespace, name)
}

func GetConnectorDescription(connectorType string, owner string) string {
	return fmt.Sprintf("%s connector for %s", connectorType, owner)
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
