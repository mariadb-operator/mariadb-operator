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
Create the name of the service account to use
*/}}
{{- define "mariadb-cluster.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "mariadb-cluster.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Validate MariaDB CR
*/}}
{{- define "mariadb-cluster.validateMariaDB" -}}
{{- if not .Values.mariadb.rootPasswordSecretKeyRef }}
{{- fail "It is required to set `.Values.mariadb.rootPasswordSecretKeyRef`" }}
{{- end }}
{{- if not .Values.mariadb.storage }}
{{- fail "It is required to set `.Values.mariadb.storage`" }}
{{- end }}
{{- if and .Values.mariadb.replication .Values.mariadb.galera }}
{{- fail "It is possible to set only one of `.Values.mariadb.replication` or `.Values.mariadb.galera`" }}
{{- end }}
{{- if and (not .Values.mariadb.replication) (not .Values.mariadb.galera) (gt (int .Values.mariadb.replicas) 1) }}
{{- fail "It is possible to specify multiple replicas in `.Values.mariadb.replicas` only when one of `.Values.mariadb.replication` or `.Values.mariadb.galera` is set" }}
{{- end }}
{{- if and .Values.mariadb.replication .Values.mariadb.galera (eq (int .Values.mariadb.replicas) 1) }}
{{- fail "It is required to specify multiple replicas in `.Values.mariadb.replicas` when one of `.Values.mariadb.replication` or `.Values.mariadb.galera` is set" }}
{{- end }}
{{- if and .Values.mariadb.maxScale .Values.mariadb.maxScaleRef }}
{{- fail "It is possible to set only one of `.Values.mariadb.maxScale` or `.Values.mariadb.maxScaleRef`" }}
{{- end }}
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
{{- if and .passwordPlugin .passwordPlugin.pluginArgSecretKeyRef (not .passwordPlugin.pluginNameSecretKeyRef) }}
{{- fail "It is required to specify `.passwordPlugin.pluginNameSecretKeyRef` when `.passwordPlugin.pluginArgSecretKeyRef` is set" }}
{{- end }}
{{- if or (and .passwordSecretKeyRef .passwordHashSecretKeyRef) (and .passwordSecretKeyRef .passwordPlugin .passwordPlugin.pluginNameSecretKeyRef) (and .passwordHashSecretKeyRef .passwordPlugin .passwordPlugin.pluginNameSecretKeyRef) }}
{{- fail "It is possible to set only one of `.passwordSecretKeyRef`, `.passwordHashSecretKeyRef` or `.passwordPlugin.pluginNameSecretKeyRef`" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Validate Grant CRs
*/}}
{{- define "mariadb-cluster.validateGrants" -}}
{{- range .Values.grants }}
{{- if not .username }}
{{- fail "It is required to specify `.username` for each Grant" }}
{{- end }}
{{- if not .privileges }}
{{- fail "It is required to specify `.privileges` for each Grant" }}
{{- end }}
{{- end }}
{{- end }}
