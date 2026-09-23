#!/usr/bin/env bash
# Render assertions for helm/dex-operator: rules the templates encode that
# `helm lint` and the values schema cannot check. Runs in the chart workflow
# (.github/workflows/chart.yml) on every change. Needs helm.
set -euo pipefail
cd "$(dirname "$0")/.."
CHART=helm/dex-operator

fail() { echo "verify-chart: FAIL: $*" >&2; exit 1; }

# The helm.sh/chart label is a valid label value (at most 63 characters,
# alphanumeric at both ends) for any chart version: the 63-character cut of
# "<name>-<version>" for a long version (a branch build's
# <version>-dev.<branch>.<date>.<time>.<sha>, or the <version>+<digest>
# helm-controller installs) can land on ".", on "_" (from "+") or on a run
# like "--.". Each version renders every object with the label it names. The
# version is set by packaging, since `helm template --version` does not apply
# to a chart directory.
pkg=$(mktemp -d)
trap 'rm -rf "$pkg"' EXIT
label_re='^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$'
while read -r v want; do
  helm package "$CHART" --version "$v" -d "$pkg" >/dev/null
  labels=$(helm template t "$pkg/$(basename "$CHART")-$v.tgz" | sed -n 's/^ *helm\.sh\/chart: *//p' | tr -d '"' | sort -u)
  [ -n "$labels" ] || fail "version $v renders no helm.sh/chart label"
  while IFS= read -r l; do
    [[ ${#l} -le 63 && $l =~ $label_re ]] || fail "version $v renders helm.sh/chart '$l', not a valid label value"
  done <<<"$labels"
  [ "$labels" = "$want" ] || fail "version $v renders helm.sh/chart '$labels', want '$want'"
done <<'VERSIONS'
0.1.0 dex-operator-0.1.0
0.17.1-dev.renovate-helm-unit.2026-09-22.14-54-24.h1a2b3c4 dex-operator-0.17.1-dev.renovate-helm-unit.2026-09-22.14-54-24
0.17.1-dev.renovate-helm-unit.2026-09-22.14-54-24+h1a2b3c4 dex-operator-0.17.1-dev.renovate-helm-unit.2026-09-22.14-54-24
0.17.1-dev.renovate-helm-unit.2026-09-22.14-54---.h1a2b3c4 dex-operator-0.17.1-dev.renovate-helm-unit.2026-09-22.14-54
VERSIONS

echo "verify-chart: ok"
