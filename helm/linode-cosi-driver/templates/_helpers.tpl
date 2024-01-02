{{/*
Expand the name of the chart.
*/}}
{{- define "linode-cosi-driver.name" -}}
  {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "linode-cosi-driver.fullname" -}}
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
{{- define "linode-cosi-driver.chart" -}}
  {{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "linode-cosi-driver.labels" -}}
helm.sh/chart: {{ include "linode-cosi-driver.chart" . }}
{{ include "linode-cosi-driver.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "linode-cosi-driver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "linode-cosi-driver.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "linode-cosi-driver.rbacName" -}}
  {{- default (include "linode-cosi-driver.fullname" .) .Values.rbac.name }}
{{- end }}

{{/*
# COSI provisioner sidecar log level.
# Values are set to the integer value, higher value means more verbose logging.
# Possible values: 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
# Default value: 4
*/}}
{{- define "linode-cosi-driver.provisionerSidecarVerbosity" }}
  {{- if (kindIs "int64" .Values.sidecar.logVerbosity) }}
    {{- if (or (ge .Values.sidecar.logVerbosity 0) (le .Values.sidecar.logVerbosity 10)) }}
      {{- .Values.sidecar.logVerbosity }}
    {{- else }}
      {{- 4 }}
    {{- end }}
  {{- else }}
    {{- 4 }}
  {{- end }}
{{- end }}

{{- define "linode-cosi-driver.otelExporterConfigMap" }}
  {{- required "value 'otelExporter.configMap.ref' required" .Values.otelExporter.configMap.ref }}
{{- end }}

{{/*
Create version of the driver image.
*/}}
{{- define "linode-cosi-driver.version" }}
  {{- $version := default .Chart.AppVersion .Values.driver.image.tag }}
  {{- if hasPrefix "v" $version }}
    {{- print $version }}
  {{- else }}
    {{- printf "v%s" $version }}
  {{- end }}
{{- end }}

{{/*
Create the full name of driver image from repository and tag.
*/}}
{{- define "linode-cosi-driver.driverImageName" }}
  {{- .Values.driver.image.repository }}:{{ include "linode-cosi-driver.version" . }}
{{- end }}

{{/*
Create the full name of provisioner sidecar image from repository and tag.
*/}}
{{- define "linode-cosi-driver.provisionerSidecarImageName" }}
  {{- .Values.sidecar.image.repository }}:{{ .Values.sidecar.image.tag }}
{{- end }}

{{/*
Create the full name of driver sidecar image from repository and tag.
*/}}
{{- define "linode-cosi-driver.secretName" }}
  {{- default (include "linode-cosi-driver.name" .) .Values.secret.ref }}
{{- end }}

{{/*
Create the full name of otel exporter sidecar image from repository and tag.
*/}}
{{- define "linode-cosi-driver.otelExporterSidecarImageName" }}
  {{- .Values.otelExporter.image.repository }}:{{ .Values.otelExporter.image.tag }}
{{- end }}

{{/*
Controlls if the observability features should be enabled.
*/}}
{{- define "linode-cosi-driver.observability" }}
  {{- $keys := keys .Values.driver.otelConfig }}
  {{- if (.Values.otelExporter.deploySidecar) }}
    {{- true }}
  {{- else }}
    {{- ne (len $keys) 0 }}
  {{- end }}
{{- end }}
