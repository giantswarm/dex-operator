package azure

import (
	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"
	absser "github.com/microsoft/kiota-abstractions-go/serialization"
	"github.com/microsoft/kiota-abstractions-go/store"
	serform "github.com/microsoft/kiota-serialization-form-go"
	serjson "github.com/microsoft/kiota-serialization-json-go"
	sermultipart "github.com/microsoft/kiota-serialization-multipart-go"
	sertext "github.com/microsoft/kiota-serialization-text-go"
	core "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

// GraphClient exposes the /applications endpoint of Microsoft Graph.
//
// It mirrors what msgraphsdk.NewGraphServiceClient sets up, without importing
// the msgraph-sdk-go root package. That package imports every Graph API
// sub-package, which makes up most of the compile time of this module and of
// every module importing it.
type GraphClient struct {
	adapter        abs.RequestAdapter
	pathParameters map[string]string
}

func NewGraphClient(auth absauth.AuthenticationProvider) (*GraphClient, error) {
	adapter, err := core.NewGraphRequestAdapterBase(auth, core.GraphClientOptions{})
	if err != nil {
		return nil, err
	}

	abs.RegisterDefaultSerializer(func() absser.SerializationWriterFactory { return serjson.NewJsonSerializationWriterFactory() })
	abs.RegisterDefaultSerializer(func() absser.SerializationWriterFactory { return sertext.NewTextSerializationWriterFactory() })
	abs.RegisterDefaultSerializer(func() absser.SerializationWriterFactory { return serform.NewFormSerializationWriterFactory() })
	abs.RegisterDefaultSerializer(func() absser.SerializationWriterFactory {
		return sermultipart.NewMultipartSerializationWriterFactory()
	})
	abs.RegisterDefaultDeserializer(func() absser.ParseNodeFactory { return serjson.NewJsonParseNodeFactory() })
	abs.RegisterDefaultDeserializer(func() absser.ParseNodeFactory { return sertext.NewTextParseNodeFactory() })
	abs.RegisterDefaultDeserializer(func() absser.ParseNodeFactory { return serform.NewFormParseNodeFactory() })

	adapter.SetBaseUrl(graphBaseURL)
	adapter.EnableBackingStore(store.BackingStoreFactoryInstance)

	return &GraphClient{
		adapter:        adapter,
		pathParameters: map[string]string{"baseurl": graphBaseURL},
	}, nil
}

func (c *GraphClient) Applications() *applications.ApplicationsRequestBuilder {
	return applications.NewApplicationsRequestBuilderInternal(c.pathParameters, c.adapter)
}
