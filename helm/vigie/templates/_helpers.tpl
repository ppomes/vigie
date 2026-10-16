{{/*
Expand the name of the chart.
*/}}
{{- define "vigie.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "vigie.fullname" -}}
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
{{- define "vigie.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "vigie.labels" -}}
helm.sh/chart: {{ include "vigie.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels for API
*/}}
{{- define "vigie.api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vigie.name" . }}-api
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for Frontend
*/}}
{{- define "vigie.frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vigie.name" . }}-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for Admin
*/}}
{{- define "vigie.admin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vigie.name" . }}-admin
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels for MySQL
*/}}
{{- define "vigie.mysql.selectorLabels" -}}
app.kubernetes.io/name: {{ include "vigie.name" . }}-mysql
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
MySQL host - depends on mode
*/}}
{{- define "vigie.mysql.host" -}}
{{- if eq .Values.mysql.mode "moco" }}
{{- printf "moco-%s-primary.%s.svc" .Values.mysql.moco.clusterName .Release.Namespace }}
{{- else }}
{{- printf "%s-mysql.%s.svc" (include "vigie.fullname" .) .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Secret name for MySQL passwords
*/}}
{{- define "vigie.mysql.secretName" -}}
{{- if .Values.mysql.existingSecret }}
{{- .Values.mysql.existingSecret }}
{{- else }}
{{- printf "%s-mysql" (include "vigie.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Secret name for app config (JWT, etc.)
*/}}
{{- define "vigie.config.secretName" -}}
{{- if .Values.config.existingSecret }}
{{- .Values.config.existingSecret }}
{{- else }}
{{- printf "%s-config" (include "vigie.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Secret name for email config
*/}}
{{- define "vigie.email.secretName" -}}
{{- if .Values.email.existingSecret }}
{{- .Values.email.existingSecret }}
{{- else }}
{{- printf "%s-email" (include "vigie.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "vigie.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
