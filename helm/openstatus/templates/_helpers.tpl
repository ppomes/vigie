{{/*
Expand the name of the chart.
*/}}
{{- define "openstatus.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "openstatus.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "openstatus.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "openstatus.labels" -}}
helm.sh/chart: {{ include "openstatus.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels for API
*/}}
{{- define "openstatus.api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openstatus.name" . }}-api
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for Frontend
*/}}
{{- define "openstatus.frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openstatus.name" . }}-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for Admin
*/}}
{{- define "openstatus.admin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openstatus.name" . }}-admin
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for MySQL
*/}}
{{- define "openstatus.mysql.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openstatus.name" . }}-mysql
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
MySQL host - depends on mode
*/}}
{{- define "openstatus.mysql.host" -}}
{{- if eq .Values.mysql.mode "moco" }}
{{- printf "moco-%s-primary.%s.svc" .Values.mysql.moco.clusterName .Release.Namespace }}
{{- else }}
{{- printf "%s-mysql.%s.svc" (include "openstatus.fullname" .) .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Secret name for MySQL passwords
*/}}
{{- define "openstatus.mysql.secretName" -}}
{{- if .Values.mysql.existingSecret }}
{{- .Values.mysql.existingSecret }}
{{- else }}
{{- printf "%s-mysql" (include "openstatus.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Secret name for app config (JWT, etc.)
*/}}
{{- define "openstatus.config.secretName" -}}
{{- if .Values.config.existingSecret }}
{{- .Values.config.existingSecret }}
{{- else }}
{{- printf "%s-config" (include "openstatus.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Secret name for email config
*/}}
{{- define "openstatus.email.secretName" -}}
{{- if .Values.email.existingSecret }}
{{- .Values.email.existingSecret }}
{{- else }}
{{- printf "%s-email" (include "openstatus.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "openstatus.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
