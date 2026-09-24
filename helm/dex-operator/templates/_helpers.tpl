{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label. A label value is at
most 63 characters and begins and ends alphanumeric: the cut of a long version
(a branch build's <version>-dev.<branch>.<date>.<time>.<sha>, or the
<version>+<digest> helm-controller installs) can land on any run of ".", "_"
(from "+") and "-", so the whole run is trimmed.
*/}}
{{- define "chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimAll "-._" -}}
{{- end -}}

{{/*
Define image tag.
*/}}
{{- define "image.tag" -}}
{{- if .Values.image.tag }}
{{- .Values.image.tag }}
{{- else }}
{{- .Chart.AppVersion }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "labels.common" -}}
app: {{ include "name" . | quote }}
{{ include "labels.selector" . }}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
application.giantswarm.io/team: {{ index .Chart.Annotations "io.giantswarm.application.team" | quote }}
helm.sh/chart: {{ include "chart" . | quote }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "labels.selector" -}}
app.kubernetes.io/name: {{ include "name" . | quote }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
{{- end -}}
