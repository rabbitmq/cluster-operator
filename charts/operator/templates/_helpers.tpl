{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "rabbitmq-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "rabbitmq-operator.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "rabbitmq-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{/*
Common labels
*/}}
{{- define "rabbitmq-operator.labels" -}}
helm.sh/chart: {{ include "rabbitmq-operator.chart" . }}
{{ include "rabbitmq-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{/*
Selector labels
*/}}
{{- define "rabbitmq-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rabbitmq-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
{{/*
Create the name of the service account to use
*/}}
{{- define "rabbitmq-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "rabbitmq-operator.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
{{/*
Create a registry image reference for use in a spec.
Includes the 'image' and 'imagePullPolicy' keys.
*/}}
{{- define "rabbitmq-operator.registryImage" -}}
image: {{ include "rabbitmq-operator.imageReference" . }}
{{ include "rabbitmq-operator.imagePullPolicy" . }}
{{- end -}}
{{/*
The most complete image reference, including the
registry address, repository, tag and digest when available.
*/}}
{{- define "rabbitmq-operator.imageReference" -}}
{{- $registry := include "rabbitmq-operator.imageRegistry" . -}}
{{- $namespace := include "rabbitmq-operator.imageNamespace" . -}}
{{- printf "%s/%s/%s" $registry $namespace .image.name -}}
{{- if .image.tag -}}
{{- printf ":%s" .image.tag -}}
{{- end -}}
{{- if .image.digest -}}
{{- printf "@%s" .image.digest -}}
{{- end -}}
{{- end -}}
{{- define "rabbitmq-operator.imageRegistry" -}}
{{- if or (and .image.useOriginalRegistry (empty .image.registry)) (and .values.useOriginalRegistry (empty .values.imageRegistry)) -}}
{{- include "rabbitmq-operator.originalImageRegistry" . -}}
{{- else -}}
{{- include "rabbitmq-operator.customImageRegistry" . -}}
{{- end -}}
{{- end -}}
{{- define "rabbitmq-operator.originalImageRegistry" -}}
{{- printf (coalesce .image.originalRegistry .values.originalImageRegistry "docker.io") -}}
{{- end -}}
{{- define "rabbitmq-operator.customImageRegistry" -}}
{{- printf (coalesce .image.registry .values.imageRegistry .values.global.imageRegistry (include "rabbitmq-operator.originalImageRegistry" .)) -}}
{{- end -}}
{{- define "rabbitmq-operator.imageNamespace" -}}
{{- if or (and .image.useOriginalNamespace (empty .image.namespace)) (and .values.useOriginalNamespace (empty .values.imageNamespace)) -}}
{{- include "rabbitmq-operator.originalImageNamespace" . -}}
{{- else -}}
{{- include "rabbitmq-operator.customImageNamespace" . -}}
{{- end -}}
{{- end -}}
{{- define "rabbitmq-operator.originalImageNamespace" -}}
{{- printf (coalesce .image.originalNamespace .values.originalImageNamespace "library") -}}
{{- end -}}
{{- define "rabbitmq-operator.customImageNamespace" -}}
{{- printf (coalesce .image.namespace .values.imageNamespace .values.global.imageNamespace (include "rabbitmq-operator.originalImageNamespace" .)) -}}
{{- end -}}
{{/*
Specify the image pull policy
*/}}
{{- define "rabbitmq-operator.imagePullPolicy" -}}
{{ $policy := coalesce .image.pullPolicy .values.global.imagePullPolicy }}
{{- if $policy -}}
imagePullPolicy: "{{ printf "%s" $policy -}}"
{{- end -}}
{{- end -}}
{{/*
Use the image pull secrets. All of the specified secrets will be used
*/}}
{{- define "rabbitmq-operator.imagePullSecrets" -}}
{{- $secrets := .Values.global.imagePullSecrets -}}
{{- range $_, $image := .Values.images -}}
{{- range $_, $s := $image.pullSecrets -}}
{{- if not $secrets -}}
{{- $secrets = list $s -}}
{{- else -}}
{{- $secrets = append $secrets $s -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- if $secrets }}
imagePullSecrets:
{{- range $secrets }}
- name: {{ . }}
{{- end }}
{{- end -}}
{{- end -}}`

{{- define "imagePullSecret" }}
{{- printf "{\"auths\": {\"%s\": {\"auth\": \"%s\"}}}" .Values.global.imageRegistry (printf "%s:%s" .Values.global.imageUsername .Values.global.imagePassword | b64enc) | b64enc }}
{{- end }}