package yaml

import (
	"encoding/json"

	yamllib "gopkg.in/yaml.v2"

	"github.com/giantswarm/microerror"
)

// MarshalWithJsonAnnotations marshals the given value to YAML, but uses the
// JSON annotations of the struct fields.
func MarshalWithJsonAnnotations(v any) ([]byte, error) {
	jsonData, jsonErr := json.Marshal(v)
	if jsonErr != nil {
		return nil, microerror.Mask(jsonErr)
	}
	var mapData map[string]interface{}
	jsonErr = json.Unmarshal(jsonData, &mapData)
	if jsonErr != nil {
		return nil, microerror.Mask(jsonErr)
	}
	data, yamlErr := yamllib.Marshal(mapData)
	return data, yamlErr
}
