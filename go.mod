module github.com/giantswarm/dex-operator

go 1.27.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.15.0
	github.com/bradleyfalzon/ghinstallation/v2 v2.19.0
	github.com/dexidp/dex v2.13.0+incompatible
	github.com/fluxcd/helm-controller/api v1.7.1
	github.com/giantswarm/apiextensions-application v0.6.0
	github.com/giantswarm/backoff/v2 v2.0.0
	github.com/giantswarm/k8smetadata v0.26.0
	github.com/giantswarm/microerror v0.4.1
	github.com/go-logr/logr v1.4.4
	github.com/go-logr/zapr v1.3.0
	github.com/google/go-github/v93 v93.0.0
	github.com/google/uuid v1.6.0
	github.com/microsoft/kiota-abstractions-go v1.12.0
	github.com/microsoft/kiota-authentication-azure-go v1.3.1
	github.com/microsoft/kiota-serialization-form-go v1.1.6
	github.com/microsoft/kiota-serialization-json-go v1.1.5
	github.com/microsoft/kiota-serialization-multipart-go v1.1.4
	github.com/microsoft/kiota-serialization-text-go v1.1.6
	github.com/microsoftgraph/msgraph-sdk-go v1.105.0
	github.com/microsoftgraph/msgraph-sdk-go-core v1.4.1
	github.com/onsi/ginkgo/v2 v2.33.1
	github.com/onsi/gomega v1.44.0
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/prometheus/client_golang v1.25.0
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	go.uber.org/zap v1.28.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.36.0
	k8s.io/apimachinery v0.37.1
	k8s.io/client-go v0.36.0
	sigs.k8s.io/cluster-api v1.11.5
	sigs.k8s.io/controller-runtime v0.24.1
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.23.3 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.13.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.10.1 // indirect
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/beevik/etree v1.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v6 v6.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-oidc v2.3.0+incompatible // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fluxcd/pkg/apis/acl v0.12.0 // indirect
	github.com/fluxcd/pkg/apis/kustomize v1.23.0 // indirect
	github.com/fluxcd/pkg/apis/meta v1.38.0 // indirect
	github.com/fluxcd/source-controller/api v1.10.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.2 // indirect
	github.com/giantswarm/micrologger v1.1.2 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/swag v0.27.1 // indirect
	github.com/go-openapi/swag/cmdutils v0.27.1 // indirect
	github.com/go-openapi/swag/conv v0.27.1 // indirect
	github.com/go-openapi/swag/fileutils v0.27.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.27.1 // indirect
	github.com/go-openapi/swag/loading v0.27.1 // indirect
	github.com/go-openapi/swag/mangling v0.27.1 // indirect
	github.com/go-openapi/swag/netutils v0.27.1 // indirect
	github.com/go-openapi/swag/pools v0.27.1 // indirect
	github.com/go-openapi/swag/stringutils v0.27.1 // indirect
	github.com/go-openapi/swag/typeutils v0.27.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.27.1 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-github/v88 v88.0.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/microsoft/kiota-http-go v1.5.6 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/pquerna/cachecontrol v0.2.0 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/russellhaering/goxmldsig v1.6.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/std-uritemplate/std-uritemplate/go/v2 v2.0.12 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.3 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ldap.v2 v2.5.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	k8s.io/apiextensions-apiserver v0.37.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260721132016-d427ff9ee9ad // indirect
	k8s.io/utils v0.0.0-20260626114624-be93311217bd // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace (
	github.com/cloudflare/circl => github.com/cloudflare/circl v1.6.5
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc => go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.72.0
	golang.org/x/net => golang.org/x/net v0.62.0
	google.golang.org/grpc => google.golang.org/grpc v1.85.0
	google.golang.org/protobuf => google.golang.org/protobuf v1.36.12
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.27.4
)

replace github.com/microsoft/kiota-http-go v1.5.4 => github.com/microsoft/kiota-http-go v1.5.6

replace github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.7 => github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0
