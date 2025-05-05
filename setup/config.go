package setup

import (
	"encoding/base64"
	"os"
	"regexp"
	"strings"

	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Oidc Oidc `yaml:"oidc,omitempty"`
}
type Oidc struct {
	Giantswarm OidcOwner `yaml:"giantswarm,omitempty"`
	Customer   OidcOwner `yaml:"customer,omitempty"`
}
type OidcOwner struct {
	Providers []OidcOwnerProvider `json:"providers,omitempty"`
}
type OidcOwnerProvider struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
}

func GetConfigFromFile(fileLocation string, base64Vars bool) (Config, error) {
	credentials := &Config{}

	file, err := os.ReadFile(fileLocation)
	if err != nil {
		return *credentials, microerror.Maskf(invalidConfigError, "Failed to get config from file: %s", err)
	}

	if base64Vars {
		return getCredentialsFromBase64Vars(file)
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return *credentials, microerror.Maskf(invalidConfigError, "Failed to get config from file: %s", err)
	}

	return *credentials, nil
}

func getCredentialsFromBase64Vars(file []byte) (Config, error) {
	var credentials = Config{
		Oidc: Oidc{
			Giantswarm: OidcOwner{
				Providers: []OidcOwnerProvider{},
			},
		},
	}

	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		// use regex to parse the line
		matcher, err := regexp.Compile(`dex_operator_(?P<x>\w+)_credential_b64='(?P<y>.*)'`)
		if err != nil {
			return Config{}, microerror.Maskf(invalidConfigError, "Failed to parse line: %s: %v", line, err)
		}
		match := matcher.FindStringSubmatch(line)
		if len(match) != 3 {
			return Config{}, microerror.Maskf(invalidConfigError, "Failed to parse line: %s", line)
		}
		// add the provider to the config
		data, err := base64.StdEncoding.DecodeString(match[2])
		if err != nil {
			return Config{}, microerror.Maskf(invalidConfigError, "Failed to decode base64: %s: %v", match[2], err)
		}
		provider := OidcOwnerProvider{Name: match[1], Credentials: string(data)}
		credentials.Oidc.Giantswarm.Providers = append(credentials.Oidc.Giantswarm.Providers, provider)
	}
	return credentials, nil
}
func getBase64VarsFromConfig(config Config) []byte {
	var lines []string
	for _, provider := range config.Oidc.Giantswarm.Providers {
		data := base64.StdEncoding.EncodeToString([]byte(provider.Credentials))
		line := "dex_operator_" + provider.Name + "_credential_b64='" + data + "'"
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n"))
}
