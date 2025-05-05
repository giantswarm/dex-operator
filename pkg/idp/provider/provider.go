package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giantswarm/dex-operator/pkg/dex"
	"github.com/giantswarm/dex-operator/pkg/key"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateOrUpdateApp(AppConfig, context.Context, dex.Connector) (ProviderApp, error)
	DeleteApp(string, context.Context) error
	GetCredentialsForAuthenticatedApp(AppConfig) (map[string]string, error)
	CleanCredentialsForAuthenticatedApp(AppConfig) error
	DeleteAuthenticatedApp(AppConfig) error
	GetName() string
	GetProviderName() string
	GetOwner() string
	GetType() string
}

type AppConfig struct {
	RedirectURI          string
	Name                 string
	IdentifierURI        string
	SecretValidityMonths int
}

type ProviderCredential struct {
	Name        string            `yaml:"name"`
	Owner       string            `yaml:"owner"`
	Credentials map[string]string `yaml:"credentials"`
	Description string            `yaml:"description"`
}

func (c ProviderCredential) GetConnectorDescription(providerDisplayName string) string {
	if c.Description != "" {
		return c.Description
	}
	return key.GetDefaultConnectorDescription(providerDisplayName, c.Owner)
}

type ProviderApp struct {
	Connector         dex.Connector
	SecretEndDateTime time.Time
}

type ProviderSecret struct {
	ClientId     string
	ClientSecret string
	EndDateTime  time.Time
}

func ReadCredentials(fileLocation string) ([]ProviderCredential, error) {
	credentials := &[]ProviderCredential{}
	// Simple security checks
	cleanPath := filepath.Clean(fileLocation)

	// Check 1: Prevent directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return nil, microerror.Mask(errors.New("security error: path contains directory traversal elements"))
	}

	// Check 2: Ensure file has expected extension
	if !strings.HasSuffix(strings.ToLower(cleanPath), ".yaml") &&
		!strings.HasSuffix(strings.ToLower(cleanPath), ".yml") {
		return nil, microerror.Mask(errors.New("security error: file must have .yaml or .yml extension"))
	}

	file, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return nil, microerror.Mask(err)
	}

	return *credentials, nil
}
