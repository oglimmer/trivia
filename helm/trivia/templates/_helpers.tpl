{{/*
Expand the name of the chart.
*/}}
{{- define "trivia.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "trivia.fullname" -}}
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
Chart label
*/}}
{{- define "trivia.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "trivia.labels" -}}
helm.sh/chart: {{ include "trivia.chart" . }}
{{ include "trivia.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "trivia.selectorLabels" -}}
app.kubernetes.io/name: {{ include "trivia.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Backend
*/}}
{{- define "trivia.backend.fullname" -}}
{{- printf "%s-backend" (include "trivia.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "trivia.backend.labels" -}}
helm.sh/chart: {{ include "trivia.chart" . }}
{{ include "trivia.backend.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: backend
{{- end }}

{{- define "trivia.backend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "trivia.name" . }}-backend
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Frontend
*/}}
{{- define "trivia.frontend.fullname" -}}
{{- printf "%s-frontend" (include "trivia.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "trivia.frontend.labels" -}}
helm.sh/chart: {{ include "trivia.chart" . }}
{{ include "trivia.frontend.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: frontend
{{- end }}

{{- define "trivia.frontend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "trivia.name" . }}-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Postgres
*/}}
{{- define "trivia.postgres.fullname" -}}
{{- printf "%s-postgres" (include "trivia.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "trivia.postgres.labels" -}}
helm.sh/chart: {{ include "trivia.chart" . }}
{{ include "trivia.postgres.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: postgres
{{- end }}

{{- define "trivia.postgres.selectorLabels" -}}
app.kubernetes.io/name: {{ include "trivia.name" . }}-postgres
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: postgres
{{- end }}

{{/*
Service account name
*/}}
{{- define "trivia.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "trivia.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name (sealed-secret holds JWT_SECRET, ADMIN_PASSWORD, POSTGRES_PASSWORD,
and optional ANTHROPIC_API_KEY)
*/}}
{{- define "trivia.secretName" -}}
{{- printf "%s-secret" (include "trivia.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
