# Connector maps

Dex-operator writes `oidc.giantswarm.connectors` as a map keyed by connector id.
Every dex-app consuming this configuration must support connector maps before
this operator version is deployed. A list-only chart cannot render these values.

Existing managed Secrets with lists remain readable. Reconciliation rewrites
them as maps even when their connector credentials have not changed. Connector
ids and configuration are preserved, so the conversion does not change Dex
subjects or rotate provider credentials.

User maps under `oidc.giantswarm.connectors` merge with the operator's map.
Each writer must use distinct ids. Lists and other non-map values under that
key still replace the whole value and are treated as conflicting user config.
`oidc.customer.connectors` remains a list and retains its conflict behavior.

On App targets, conflicting user config for an owner the operator manages
removes the managed configuration. On HelmRelease targets, the managed Secret
remains available because `valuesFrom` references it; the later values source
takes precedence. HelmRelease `spec.valuesFrom` remains owned by Git.
