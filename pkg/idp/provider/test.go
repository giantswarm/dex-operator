package provider

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GetTestConfig() AppConfig {
	return AppConfig{RedirectURI: "hello.io", Name: "test"}
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
