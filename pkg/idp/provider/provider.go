package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/dexidp/dex/server"
	"gopkg.in/yaml.v2"
)

type Provider interface {
	CreateApp(AppConfig) (Connector, error)
	DeleteApp(string) error
}

type AppConfig struct {
	RedirectURI string
	Name        string
}

type ProviderCredential struct {
	Name        string            `yaml:"name"`
	Credentials map[string]string `yaml:"credentials"`
}

func ReadCredentials(fileLocation string) ([]ProviderCredential, error) {
	credentials := &[]ProviderCredential{}

	file, err := os.ReadFile(fileLocation)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(file, credentials); err != nil {
		return nil, err
	}

	return *credentials, nil
}

/*
Connector type and below code is copied from dex (github.com/dexidp/dex/cmd/dex)
The reason we reimplement it here instead of importing is that this type is defined
in the main package and can't be imported
*/

// Connector is a magical type that can unmarshal YAML dynamically. The
// Type field determines the connector type, which is then customized for Config.
type Connector struct {
	Type string `json:"type"`
	Name string `json:"name"`
	ID   string `json:"id"`

	Config server.ConnectorConfig `json:"config"`
}

// UnmarshalJSON allows Connector to implement the unmarshaler interface to
// dynamically determine the type of the connector config.
func (c *Connector) UnmarshalJSON(b []byte) error {
	var conn struct {
		Type string `json:"type"`
		Name string `json:"name"`
		ID   string `json:"id"`

		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(b, &conn); err != nil {
		return fmt.Errorf("parse connector: %v", err)
	}
	f, ok := server.ConnectorsConfig[conn.Type]
	if !ok {
		return fmt.Errorf("unknown connector type %q", conn.Type)
	}

	connConfig := f()
	if len(conn.Config) != 0 {
		data := []byte(conn.Config)
		if isExpandEnvEnabled() {
			// Caution, we're expanding in the raw JSON/YAML source. This may not be what the admin expects.
			data = []byte(os.ExpandEnv(string(conn.Config)))
		}
		if err := json.Unmarshal(data, connConfig); err != nil {
			return fmt.Errorf("parse connector config: %v", err)
		}
	}
	*c = Connector{
		Type:   conn.Type,
		Name:   conn.Name,
		ID:     conn.ID,
		Config: connConfig,
	}
	return nil
}

// isExpandEnvEnabled returns if os.ExpandEnv should be used for each storage and connector config.
// Disabling this feature avoids surprises e.g. if the LDAP bind password contains a dollar character.
// Returns false if the env variable "DEX_EXPAND_ENV" is a falsy string, e.g. "false".
// Returns true if the env variable is unset or a truthy string, e.g. "true", or can't be parsed as bool.
func isExpandEnvEnabled() bool {
	enabled, err := strconv.ParseBool(os.Getenv("DEX_EXPAND_ENV"))
	if err != nil {
		// Unset, empty string or can't be parsed as bool: Default = true.
		return true
	}
	return enabled
}
