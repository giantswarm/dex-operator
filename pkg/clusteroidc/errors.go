package clusteroidc

import "github.com/giantswarm/microerror"

var oidcFlagsConfigNotFoundError = &microerror.Error{
	Kind: "oidcFlagsConfigNotFound",
}

func IsOIDCFlagsConfigNotFound(err error) bool {
	return microerror.Cause(err) == oidcFlagsConfigNotFoundError
}

var oidcFlagsNotFoundError = &microerror.Error{
	Kind: "oidcFlagsNotFound",
}

func IsOIDCFlagsNotFound(err error) bool {
	return microerror.Cause(err) == oidcFlagsNotFoundError
}
