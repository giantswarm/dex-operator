/*
In order to manage dex applications in azure, each dex-operator will need it's own credential secret.
When a dex-operator application is registered in an azure tenant, a new secret can be added to it for each instance of dex-operator.
So one for each installation the operator should manage dex-apps on.
Creating a new secret can be done manually via azure portal. However, the following script can be used as well
and may be convenient when doing it for more installations in bulk.

The following environment variables need to be set:

INSTALLATION - name for the credential - use installation name of the installation the credentials should be used for.
CREDENTIAL_FILE - location of config secret patch file containing credentials currently used by dex-operator. Can be for the same installation (update case) or for another one (create case).
ACTION - "create" to create new credentials for dex-operator or "clean" to delete other credentials for the installation that are not in use anymore

The output matches the giantswarm config format.

Example to run it for an installation:

export ACTION=create
export CREDENTIAL_FILE=tmp
export INSTALLATION=test
go run scripts/dex-operator-azure-credentials.go > $INSTALLATION


Example to run it for a number of giant swarm installations:

export ACTION=create
export CREDENTIAL_FILE=tmp
for i in $(opsctl list installations --pipeline testing --short); do
   export INSTALLATION=$i
   go run scripts/dex-operator-azure-credentials.go > $i"
done
*/

// TODO this script should be replaced by general tool with flags
package main

import (
	"fmt"
	"giantswarm/dex-operator/pkg/idp/provider/azure"
	"giantswarm/dex-operator/setup"
	"os"
)

const (
	InstallationKey = "INSTALLATION"
	CredentialKey   = "CREDENTIAL_FILE"
	ActionKey       = "ACTION"
)

func getConfig() (setup.Setup, error) {
	installation := os.Getenv(InstallationKey)
	if installation == "" {
		return setup.Setup{}, fmt.Errorf("%s must not be empty", InstallationKey)
	}
	credentialFile := os.Getenv(CredentialKey)
	if credentialFile == "" {
		return setup.Setup{}, fmt.Errorf("%s must not be empty", CredentialKey)
	}
	action := os.Getenv(ActionKey)
	if action == "" {
		return setup.Setup{}, fmt.Errorf("%s must not be empty", ActionKey)
	}

	return setup.Setup{
		Installation:   installation,
		CredentialFile: credentialFile,
		Action:         action,
		Provider:       azure.ProviderName,
		AppName:        "dex-operator",
	}, nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Printf("failed due to error %s", err)
		os.Exit(1)
	}
}

func run() error {
	command, err := getConfig()
	if err != nil {
		return err
	}
	return setup.CredentialSetup(command)
}
