package azure

import (
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
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

func PrintOdataError(err error) string {
	switch typed := err.(type) {
	case *odataerrors.ODataError:
		if terr := typed.GetError(); terr != nil {
			return fmt.Sprintf("error: %s\n code: %s\n msg: %s", typed.Error(), *terr.GetCode(), *terr.GetMessage())
		} else {
			return fmt.Sprintf("error: %s", typed.Error())
		}
	default:
		return fmt.Sprintf("%T > error: %#v", err, err)
	}
}
