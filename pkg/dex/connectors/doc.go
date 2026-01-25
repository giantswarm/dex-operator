// Package connectors contains local copies of Dex connector configuration types.
//
// These types are copied from github.com/dexidp/dex to avoid dependency issues
// with Dex's broken Go module versioning. Dex uses v2.x version tags but doesn't
// follow Go's semantic import versioning (no /v2 in module path), making it
// impossible to properly depend on recent versions.
//
// See: https://github.com/dexidp/dex/issues/4222
//
// The types are used purely for YAML serialization of connector configs that
// are passed to Dex instances. The actual Dex instances validate these configs
// at runtime.
//
// When updating these types, refer to the Dex source code:
// https://github.com/dexidp/dex/tree/main/connector
package connectors
