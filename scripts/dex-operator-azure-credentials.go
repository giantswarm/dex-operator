/*
In order to manage dex applications in azure, each dex-operator will need it's own credential secret.
When a dex-operator application is registered in an azure tenant, a new secret can be added to it for each instance of dex-operator.
So one for each installation the operator should manage dex-apps on.
Creating a new secret can be done manually via azure portal. However, the following script can be used as well
and may be convenient when doing it for more installations in bulk.

The following environment variables need to be set:

TENANT_ID - the tenant in which the dex-operator application is present
CLIENT_ID - client id of the dex-operator application
CLIENT_SECRET - existing client secret of the dex-operator application
INSTALLATION - name for the credential - recommended to use installation name of the installation the credentials should be used for.

The output matches the giantswarm config format.

Example to run it for an installation:

export TENANT_ID=x
export CLIENT_ID=y
export CLIENT_SECRET=z
export INSTALLATION=test
go run scripts/dex-operator-azure-credentials.go > $INSTALLATION


Example to run it for a number of giant swarm installations:

export TENANT_ID=x
export CLIENT_ID=y
export CLIENT_SECRET=z
for i in $(opsctl list installations --pipeline testing --short); do
   export INSTALLATION=$i
   go run scripts/dex-operator-azure-credentials.go > $i"
done
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"giantswarm/dex-operator/pkg/idp/provider/azure"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

const (
	TenantIDKey     = "TENANT_ID"
	ClientIDKey     = "CLIENT_ID"
	ClientSecretKey = "CLIENT_SECRET"
	InstallationKey = "INSTALLATION"
	AppName         = "dex-operator"
)

type Credentials struct {
	clientID     string
	clientSecret string
	tenantID     string
}

func main() {
	credentials, err := AzureCredentials()
	if err != nil {
		fmt.Printf("Failed to get azure credentials due to error %s", err)
		os.Exit(1)
	}
	fmt.Print(credentials.asConfigPatch())
}

func AzureCredentials() (Credentials, error) {
	client, tenantID, err := getAzureClient()
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to get azure client due to error %s", err)
	}
	installation, err := getInstallation()
	if err != nil {
		return Credentials{}, fmt.Errorf("failed due to error %s", err)
	}
	app, err := getApp(client)
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to get app %s due to error %s", AppName, err)
	}
	clientID, clientSecret, err := createSecretForInstallation(client, installation, app)
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to create secret for installation %s due to error %s", installation, err)
	}
	c := Credentials{
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
	}
	return c, nil
}

func createSecretForInstallation(client *msgraphsdk.GraphServiceClient, installation string, app models.Applicationable) (string, string, error) {

	secretName := fmt.Sprintf("%s-%s", AppName, installation)
	_, err := azure.GetSecret(app, secretName)
	if err == nil {
		return "", "", fmt.Errorf("secret with name %s already exists. Not creating a new secret", secretName)
	} else if !azure.IsNotFound(err) {
		return "", "", err
	}
	id := app.GetId()
	if id == nil {
		return "", "", fmt.Errorf("could not find ID of app %s", AppName)
	}
	secret, err := client.ApplicationsById(*id).AddPassword().Post(context.Background(), azure.GetSecretCreateRequestBody(secretName, 24), nil)
	if err != nil {
		return "", "", errors.New(azure.PrintOdataError(err))
	}
	clientSecret := secret.GetSecretText()
	if clientSecret == nil {
		return "", "", fmt.Errorf("could not find contents of secret %s", secretName)
	}
	clientId := app.GetAppId()
	if clientId == nil {
		return "", "", fmt.Errorf("could not find client id of app %s", AppName)
	}
	return *clientId, *clientSecret, nil
}

func getApp(client *msgraphsdk.GraphServiceClient) (models.Applicationable, error) {
	result, err := client.Applications().Get(context.Background(), azure.GetAppGetRequestConfig(AppName))
	if err != nil {
		return nil, errors.New(azure.PrintOdataError(err))
	}
	count := result.GetOdataCount()
	if *count == 0 {
		return nil, fmt.Errorf("no application with name %s exists", AppName)
	} else if *count != 1 {
		return nil, fmt.Errorf("expected 1 application %s, got %v", AppName, count)
	}
	appList := result.GetValue()
	if len(appList) != 1 {
		return nil, fmt.Errorf("expected 1 application %s, got %v", AppName, len(appList))
	}
	return appList[0], nil
}

func getInstallation() (string, error) {
	installation := os.Getenv(InstallationKey)
	if installation == "" {
		return "", fmt.Errorf("%s must not be empty", InstallationKey)
	}
	return installation, nil
}

func getAzureClient() (*msgraphsdk.GraphServiceClient, string, error) {
	var tenantID, clientID, clientSecret string
	{
		if tenantID = os.Getenv(TenantIDKey); tenantID == "" {
			return nil, "", fmt.Errorf("%s must not be empty", TenantIDKey)
		}
		if clientID = os.Getenv(ClientIDKey); clientID == "" {
			return nil, "", fmt.Errorf("%s must not be empty", ClientIDKey)
		}
		if clientSecret = os.Getenv(ClientSecretKey); clientSecret == "" {
			return nil, "", fmt.Errorf("%s must not be empty", ClientSecretKey)
		}
	}
	var client *msgraphsdk.GraphServiceClient
	{
		cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		if err != nil {
			return nil, "", err
		}
		auth, err := azauth.NewAzureIdentityAuthenticationProviderWithScopes(cred, azure.ProviderScope())
		if err != nil {
			return nil, "", err
		}
		adapter, err := msgraphsdk.NewGraphRequestAdapter(auth)
		if err != nil {
			return nil, "", err
		}
		client = msgraphsdk.NewGraphServiceClient(adapter)
		if err != nil {
			return nil, "", err
		}
	}
	return client, tenantID, nil
}

func (c *Credentials) asConfigPatch() string {
	return fmt.Sprintf(`oidc:
  giantswarm:
    providers:
    - credentials: |-
        client-id: %s
        client-secret: %s
        tenant-id: %s
      name: ad`, c.clientID, c.clientSecret, c.tenantID)
}
