package key

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AppLabel                     = "app.kubernetes.io/name"
	DexAppLabelValue             = "dex-app"
	DexConfigName                = "default-dex-config"
	DexOperatorFinalizer         = "dex-operator.finalizers.giantswarm.io"
	DexOperatorLabelValue        = "dex-operator"
	ClusterValuesConfigmapSuffix = "cluster-values"
	ClusterValuesConfigMapKey    = "values"
	BaseDomainKey                = "baseDomain"
	DexResourceURI               = "https://dex.giantswarm.io"
)

const (
	SecretValidityMonths = 6
)

func DexLabelSelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			AppLabel: DexAppLabelValue,
		},
	}
}

func GetIdpAppName(installation string, namespace string, name string) string {
	return fmt.Sprintf("%s-%s-%s", installation, namespace, name)
}

func GetConnectorDescription(connectorType string, owner string) string {
	return fmt.Sprintf("%s connector for %s", connectorType, owner)
}

func GetRedirectURI(baseDomain string) string {
	return fmt.Sprintf("https://dex.g8s.%s/callback", baseDomain)
}

func GetIdentifierURI(name string) string {
	return fmt.Sprintf("https://dex.giantswarm.io/%s", name)
}
