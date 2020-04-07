{{/* vim: set filetype=mustache: */}}

{{- define "imagePullSecret" }}
{{- printf "{\"auths\": {\"%s\": {\"auth\": \"%s\"}}}" .Values.global.imageRegistry (printf "%s:%s" .Values.global.imageUsername .Values.global.imagePassword | b64enc) | b64enc }}
{{- end }}
