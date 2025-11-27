{{- define "cloud-provider-config" -}}
{{ .Values.cloudProviderConfig | toYaml | indent 0}}
{{- end -}}

{{- define "cloud-provider-disk-config" -}}
{{ .Values.cloudProviderDiskConfig | toYaml | indent 0 }}
{{- end -}}
