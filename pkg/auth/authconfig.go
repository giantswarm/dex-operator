package auth

import (
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/k8smetadata/pkg/label"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type authConfig struct {
	name              string
	cluster           string
	namespace         string
	managementCluster string
	adminGroups       []string
	apiServerPort     int
}
type AuthConfigValues struct {
	ManagementCluster string     `yaml:"managementCluster,omitempty"`
	Bindings          []Binding  `yaml:"bindings,omitempty"`
	Kubernetes        Kubernetes `yaml:"kubernetes,omitempty"`
}
type Binding struct {
	Role   string   `yaml:"role,omitempty"`
	Groups []string `yaml:"groups,omitempty"`
}
type Kubernetes struct {
	API KubernetesAPI `yaml:"api,omitempty"`
}
type KubernetesAPI struct {
	Port int `yaml:"port,omitempty"`
}

func getAuthConfigMap(config authConfig) (*corev1.ConfigMap, error) {
	values := &AuthConfigValues{
		ManagementCluster: config.managementCluster,
		Bindings: []Binding{
			{
				Role:   key.AdminRoleName,
				Groups: config.adminGroups,
			},
		},
		Kubernetes: Kubernetes{
			API: KubernetesAPI{
				Port: config.apiServerPort,
			},
		},
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.name,
			Namespace: config.namespace,
			Labels: map[string]string{
				label.ManagedBy: key.DexOperatorLabelValue,
			},
		},
		Data: map[string]string{
			key.ValuesConfigMapKey: string(data),
		},
	}, nil
}
