{{- define "resources.konfigure-operator" -}}
requests:
{{ toYaml .Values.resources.requests | indent 2 -}}
{{ if eq (include "resource.vpa.enabled" .) "false" }}
limits:
{{ toYaml .Values.resources.limits | indent 2 -}}
{{- end -}}
{{- end -}}
