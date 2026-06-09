{{/*
Expand the name of the chart.
*/}}
{{- define "nutrition-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name (release name + chart name).
*/}}
{{- define "nutrition-api.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create the chart name + version label string.
*/}}
{{- define "nutrition-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every object.
*/}}
{{- define "nutrition-api.labels" -}}
helm.sh/chart: {{ include "nutrition-api.chart" . }}
{{ include "nutrition-api.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels — must be stable across upgrades since they're used by the
Deployment.spec.selector (which is immutable).
*/}}
{{- define "nutrition-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nutrition-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name to use.
*/}}
{{- define "nutrition-api.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "nutrition-api.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name the Deployment binds to — either the externally-managed one
the user supplied via existingSecret, or the chart-rendered one.
*/}}
{{- define "nutrition-api.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- include "nutrition-api.fullname" . }}
{{- end }}
{{- end }}

{{/*
ConfigMap name (always chart-rendered).
*/}}
{{- define "nutrition-api.configMapName" -}}
{{- include "nutrition-api.fullname" . }}
{{- end }}

{{/*
Image reference. Falls back to .Chart.AppVersion when .Values.image.tag is empty.
*/}}
{{- define "nutrition-api.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
