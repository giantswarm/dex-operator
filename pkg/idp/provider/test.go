package provider

import (
	"github.com/giantswarm/dex-operator/pkg/app"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GetTestConfig() app.Config {
	return app.Config{RedirectURI: "hello.io", Name: "test", SecretValidityMonths: 6}
}

func GetTestCredential() ProviderCredential {
	return ProviderCredential{
		Name:  "test",
		Owner: "test",
		Credentials: map[string]string{
			"id": "123",
		},
	}
}

func GetTestLogger() *logr.Logger {
	l := ctrl.Log.WithName("test")
	return &l
}
