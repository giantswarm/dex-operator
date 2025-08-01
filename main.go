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

package main

import (
	"flag"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/giantswarm/dex-operator/controllers"
	"github.com/giantswarm/dex-operator/pkg/key"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(capi.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		baseDomain               string
		issuerAddress            string
		enableLeaderElection     bool
		idpCredentials           string
		managementCluster        string
		metricsAddr              string
		probeAddr                string
		giantswarmWriteAllGroups string
		customerWriteAllGroups   string
		enableSelfRenewal        bool
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&idpCredentials, "idp-credentials-file", "/home/.idp/credentials", "The location of the idp credentials file.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.StringVar(&baseDomain, "base-domain", "", "Domain for the dex callback address, e.g. customer.gigantic.io.")
	flag.StringVar(&issuerAddress, "issuer-address", "", "URL of the identity issuer")
	flag.StringVar(&managementCluster, "management-cluster", "", "Name of the management cluster.")
	flag.StringVar(&giantswarmWriteAllGroups, "giantswarm-write-all-groups", "", "Comma separated list of giantswarm admin groups.")
	flag.StringVar(&customerWriteAllGroups, "customer-write-all-groups", "", "Comma separated list of customer admin groups.")
	flag.BoolVar(&enableSelfRenewal, "enable-self-renewal", false, "Enable automatic self-renewal of operator credentials")
	opts := zap.Options{
		Development: false,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Parse groups (handle empty strings)
	var gsGroups, customerGroups []string
	if giantswarmWriteAllGroups != "" {
		gsGroups = strings.Split(giantswarmWriteAllGroups, ",")
	}
	if customerWriteAllGroups != "" {
		customerGroups = strings.Split(customerWriteAllGroups, ",")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "bf139543.giantswarm",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.AppReconciler{
		BaseDomain:               baseDomain,
		IssuerAddress:            issuerAddress,
		ManagementCluster:        managementCluster,
		Client:                   mgr.GetClient(),
		Log:                      ctrl.Log.WithName("controllers").WithName("App"),
		Scheme:                   mgr.GetScheme(),
		LabelSelector:            key.DexLabelSelector(),
		ProviderCredentials:      idpCredentials,
		GiantswarmWriteAllGroups: gsGroups,
		CustomerWriteAllGroups:   customerGroups,
		EnableSelfRenewal:        enableSelfRenewal,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "App")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
