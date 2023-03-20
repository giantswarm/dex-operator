/*
Copyright 2022.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/giantswarm/microerror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/idp"
	"github.com/giantswarm/dex-operator/pkg/key"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("App controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		AppName      = "test-app"
		AppNamespace = "test-namespace"
		SecretName   = "test-secret"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250

		dexConfigSecretKey  = "default"
		expectedContentFile = "test-data/default-dex-config.json"
	)

	Context("When reconciling an app", func() {
		It("Should create app config secret", func() {
			ctx := context.Background()
			By("Creating the namespace")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: AppNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())

			By("Creating a new app")
			app := &v1alpha1.App{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "application.giantswarm.io/v1alpha1",
					Kind:       "App",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      AppName,
					Namespace: AppNamespace,
				},
				Spec: v1alpha1.AppSpec{
					Config: v1alpha1.AppSpecConfig{
						ConfigMap: v1alpha1.AppSpecConfigConfigMap{
							Name:      key.DexConfigName,
							Namespace: AppNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).Should(Succeed())
			appLookupKey := types.NamespacedName{Name: AppName, Namespace: AppNamespace}
			createdApp := &v1alpha1.App{}

			// We'll need to retry getting this newly created App, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, appLookupKey, createdApp)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdApp.Spec.Config.ConfigMap.Name).Should(Equal(key.DexConfigName))

			By("Checking the app extra config is still empty")
			Consistently(func() (int, error) {
				err := k8sClient.Get(ctx, appLookupKey, createdApp)
				if err != nil {
					return -1, err
				}
				return len(createdApp.Spec.ExtraConfigs), nil
			}, duration, interval).Should(Equal(0))

			By("Adding the label to the app")
			app.SetLabels(map[string]string{key.AppLabel: key.DexAppLabelValue})
			Expect(k8sClient.Update(ctx, app)).Should(Succeed())

			createdApp = &v1alpha1.App{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, appLookupKey, createdApp)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdApp.GetLabels()[key.AppLabel]).Should(Equal(key.DexAppLabelValue))

			createdApp = &v1alpha1.App{}
			By("Checking the app extra config was added")
			Eventually(func() ([]v1alpha1.AppExtraConfig, error) {
				err := k8sClient.Get(ctx, appLookupKey, createdApp)
				if err != nil {
					return nil, microerror.Mask(err)
				}
				return createdApp.Spec.ExtraConfigs, nil
			}, duration, interval).Should(Equal([]v1alpha1.AppExtraConfig{
				idp.GetDexSecretConfig(AppNamespace),
			}))

			By("Checking the dex config secret was created")
			secretLookupKey := types.NamespacedName{Name: key.DexConfigName, Namespace: AppNamespace}
			createdSecret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, createdSecret)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdSecret.Data).ShouldNot(BeNil())
			Expect(createdSecret.Data).Should(HaveKey(dexConfigSecretKey))

			expectedContent, err := os.ReadFile(expectedContentFile)
			Expect(err).NotTo(HaveOccurred())

			createdSecretDexConfigData := createdSecret.Data[dexConfigSecretKey]
			Expect(createdSecretDexConfigData).Should(Equal([]byte(strings.TrimSpace(string(expectedContent)))))

			var dexConfig dex.DexConfig
			Expect(json.Unmarshal(createdSecretDexConfigData, &dexConfig)).To(Succeed())
			Expect(dexConfig.Oidc.Giantswarm).NotTo(BeNil())
			Expect(dexConfig.Oidc.Giantswarm.Connectors).To(HaveLen(1))
			Expect(dexConfig.Oidc.Customer).To(BeNil())
			// TODO check what is inside the secret

			By("Deleting the app")
			Expect(k8sClient.Delete(ctx, app)).Should(Succeed())

			createdApp = &v1alpha1.App{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, appLookupKey, createdApp)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking the dex config secret was deleted")
			createdSecret = &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, createdSecret)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

})
