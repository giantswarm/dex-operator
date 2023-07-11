package auth

import (
	"context"
	"testing"

	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/k8smetadata/pkg/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name                  string
		managementClusterName string
		writeAllGroups        []string
		clusterName           string
		existingConfigMap     *corev1.ConfigMap
		expectedConfig        string
	}{
		{
			name:                  "case 1: Create CM",
			managementClusterName: "mc",
			clusterName:           "wc",
			writeAllGroups:        []string{"group_a", "group_b"},
			expectedConfig:        "managementCluster: mc\nbindings:\n- role: cluster-admin\n  groups:\n  - group_a\n  - group_b\n",
		},
		{
			name:                  "case 1: MC case, skip creation",
			managementClusterName: "mc",
			clusterName:           "mc",
			writeAllGroups:        []string{"group_a", "group_b"},
			expectedConfig:        "",
		},
		{
			name:                  "case 2: Update CM",
			managementClusterName: "mc",
			clusterName:           "wc",
			writeAllGroups:        []string{"group_a", "group_b"},
			existingConfigMap: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.GetAuthConfigName("wc"),
					Namespace: "example",
				},
				Data: map[string]string{
					key.ValuesConfigMapKey: "managementCluster: mc\nbindings:\n- role: cluster-admin\n  groups:\n  - group_x\n  - group_y\n",
				},
			},
			expectedConfig: "managementCluster: mc\nbindings:\n- role: cluster-admin\n  groups:\n  - group_a\n  - group_b\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			ctx := context.Background()

			fakeClientBuilder := fake.NewClientBuilder()
			if tc.existingConfigMap != nil {
				fakeClientBuilder.WithObjects(tc.existingConfigMap)
			}

			service := Service{
				Client: fakeClientBuilder.Build(),
				log:    ctrl.Log.WithName("test"),
				app: &v1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "example",
						Labels:    map[string]string{label.Cluster: tc.clusterName},
					},
				},
				writeAllGroups:        tc.writeAllGroups,
				managementClusterName: tc.managementClusterName,
			}

			err := service.Reconcile(ctx)
			if err != nil {
				t.Fatal(err)
			}
			result := &corev1.ConfigMap{}
			if err := service.Client.Get(ctx, types.NamespacedName{
				Name:      key.GetAuthConfigName(tc.clusterName),
				Namespace: "example"},
				result); err != nil {
				if !apierrors.IsNotFound(err) {
					t.Fatal(err)
				}
			}

			if result.Data[key.ValuesConfigMapKey] != tc.expectedConfig {
				t.Fatalf("Expected %s, got %s", tc.expectedConfig, result.Data[key.ValuesConfigMapKey])
			}
		})
	}
}
