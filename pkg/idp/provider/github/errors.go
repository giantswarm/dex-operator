package github

import (
	"github.com/giantswarm/microerror"
)

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidcConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var notFoundError = &microerror.Error{
	Kind: "notFoundError",
}

// IsNotFound asserts notFoundError.
func IsNotFound(err error) bool {
	return microerror.Cause(err) == notFoundError
}

var requestFailedError = &microerror.Error{
	Kind: "requestFailedError",
}

// IsRequestFailed asserts requestFailedError.
func IsRequestFailed(err error) bool {
	return microerror.Cause(err) == requestFailedError
}
