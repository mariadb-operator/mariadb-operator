{{/*
Expand the name of the chart.
*/}}
{{- define "mariadb-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mariadb-cluster.fullname" -}}
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
{{- define "mariadb-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "mariadb-cluster.labels" -}}
helm.sh/chart: {{ include "mariadb-cluster.chart" . }}
{{ include "mariadb-cluster.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "mariadb-cluster.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mariadb-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Validate Database CRs
*/}}
{{- define "mariadb-cluster.validateDatabases" -}}
{{- range .Values.databases }}
{{- if not .name }}
{{- fail "It is required to specify `.name` for each Database" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Validate User CRs
*/}}
{{- define "mariadb-cluster.validateUsers" -}}
{{- range .Values.users }}
{{- if not .name }}
{{- fail "It is required to specify `.name` for each User" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Validate Grant CRs
*/}}
{{- define "mariadb-cluster.validateGrants" -}}
{{- range .Values.grants }}
{{- if not .name }}
{{- fail "It is required to specify `.name` for each Grant" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Validate Backup CRs
*/}}
{{- define "mariadb-cluster.validateBackups" -}}
{{- range .Values.backups }}
{{- if not .name }}
{{- fail "It is required to specify `.name` for each Backup" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Validate PhysicalBackup CRs
*/}}
{{- define "mariadb-cluster.validatePhysicalBackups" -}}
{{- range .Values.physicalBackups }}
{{- if not .name }}
{{- fail "It is required to specify `.name` for each PhysicalBackup" }}
{{- end }}
{{- end }}
{{- end }}
