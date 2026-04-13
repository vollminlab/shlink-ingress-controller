{{- define "shlink-ingress-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "shlink-ingress-controller.fullname" -}}
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

{{- define "shlink-ingress-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "shlink-ingress-controller.labels" -}}
helm.sh/chart: {{ include "shlink-ingress-controller.chart" . }}
{{ include "shlink-ingress-controller.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: {{ include "shlink-ingress-controller.name" . }}
env: production
category: apps
{{- end }}

{{- define "shlink-ingress-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "shlink-ingress-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "shlink-ingress-controller.serviceAccountName" -}}
{{ include "shlink-ingress-controller.fullname" . }}
{{- end }}
