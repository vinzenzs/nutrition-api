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

{{/*
garmin-bridge — the opt-in Garmin sync sidecar. Its objects share the chart's
naming/labelling but carry a "-garmin-bridge" suffix and their own selector so
they never collide with the backend's.
*/}}
{{- define "garmin-bridge.fullname" -}}
{{- printf "%s-garmin-bridge" (include "nutrition-api.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "garmin-bridge.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nutrition-api.name" . }}-garmin-bridge
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "garmin-bridge.labels" -}}
helm.sh/chart: {{ include "nutrition-api.chart" . }}
{{ include "garmin-bridge.selectorLabels" . }}
app.kubernetes.io/component: garmin-bridge
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Secret name the bridge binds to — the externally-managed one if supplied,
else the chart-rendered bridge Secret.
*/}}
{{- define "garmin-bridge.secretName" -}}
{{- if .Values.garminBridge.existingSecret }}
{{- .Values.garminBridge.existingSecret }}
{{- else }}
{{- include "garmin-bridge.fullname" . }}
{{- end }}
{{- end }}

{{/*
Bridge image reference. tag falls back to the chart appVersion when empty.
*/}}
{{- define "garmin-bridge.image" -}}
{{- $tag := default .Chart.AppVersion .Values.garminBridge.image.tag }}
{{- printf "%s:%s" .Values.garminBridge.image.repository $tag }}
{{- end }}

{{/*
The in-cluster base URL the bridge posts to. Defaults to the backend Service
DNS when garminBridge.nutritionApiUrl is left empty.
*/}}
{{- define "garmin-bridge.nutritionApiUrl" -}}
{{- if .Values.garminBridge.nutritionApiUrl }}
{{- .Values.garminBridge.nutritionApiUrl }}
{{- else }}
{{- printf "http://%s:%v" (include "nutrition-api.fullname" .) .Values.service.port }}
{{- end }}
{{- end }}
