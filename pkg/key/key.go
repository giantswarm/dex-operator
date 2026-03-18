package key

import (
	"fmt"
	"time"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppLabel                     = "app.kubernetes.io/name"
	AdminRoleName                = "cluster-admin"
	DexAppLabelValue             = "dex-app"
	AuthConfigName               = "default-auth-config"
	DexConfigName                = "default-dex-config"
	DexOperatorFinalizer         = "dex-operator.finalizers.giantswarm.io/app-controller"
	DexOperatorLabelValue        = "dex-operator"
	ClusterValuesConfigmapSuffix = "cluster-values"
	ValuesConfigMapKey           = "values"
	MCDexAppDefaultName          = "dex-app"
	MCDexAppDefaultNamespace     = "giantswarm"
	BaseDomainKey                = "baseDomain"
	ConnectorsKey                = "connectors"
	DexResourceURI               = "https://dex.giantswarm.io"
	OwnerGiantswarm              = "giantswarm"
	OwnerCustomer                = "customer"
	OwnerGiantswarmDisplayName   = "Giant Swarm"
	OwnerCustomerDisplayName     = "Customer"

	SecretValidityMonths       = 3
	CredentialRenewalThreshold = 30 * 24 * time.Hour // 30 days before expiry

	// DexSecretConfigPriority is the priority for the dex secret config in App CR extraConfigs
	DexSecretConfigPriority = 25

	// HelmRelease-specific constants
	// Note: HelmRelease does not have a priority system like App CR
	// Values are merged in order, with later values overwriting earlier ones
	MCDexHelmReleaseDefaultName = "dex-app"
)

// IsManagementClusterDexApp checks if the app is the management cluster dex app
// Moved from AppReconciler as it has no dependencies on reconciler fields
func IsManagementClusterDexApp(app *v1alpha1.App) bool {
	return app.Name == MCDexAppDefaultName &&
		app.Namespace == MCDexAppDefaultNamespace
}

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

func MCDexHelmReleaseDefaultNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      MCDexHelmReleaseDefaultName,
		Namespace: MCDexAppDefaultNamespace,
	}
}

// IsManagementClusterDexHelmRelease checks if the HelmRelease is the management cluster dex-app
func IsManagementClusterDexHelmRelease(name, namespace string) bool {
	return name == MCDexHelmReleaseDefaultName && namespace == MCDexAppDefaultNamespace
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

func GetIdpAppName(managementClusterName string, namespace string, name string) string {
	return fmt.Sprintf("%s-%s-%s", managementClusterName, namespace, name)
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

func GetDexOperatorName(managementClusterName string) string {
	return fmt.Sprintf("dex-operator-%s", managementClusterName)
}
