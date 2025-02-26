{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "konfigure-operator.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "konfigure-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "konfigure-operator.labels" -}}
{{ include "konfigure-operator.selectorLabels" . }}
app: {{ include "konfigure-operator.name" . | quote }}
application.giantswarm.io/branch: {{ .Chart.AppVersion | replace "#" "-" | replace "/" "-" | replace "." "-" | trunc 63 | trimSuffix "-" | quote }}
application.giantswarm.io/commit: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
application.giantswarm.io/team: {{ index .Chart.Annotations "application.giantswarm.io/team" | quote }}
helm.sh/chart: {{ include "konfigure-operator.chart" . | quote }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "konfigure-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "konfigure-operator.name" . | quote }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
{{- end -}}

{{- define "resource.vpa.enabled" -}}
{{- if and (.Capabilities.APIVersions.Has "autoscaling.k8s.io/v1") (.Values.verticalPodAutoscaler.enabled) }}true{{ else }}false{{ end }}
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
