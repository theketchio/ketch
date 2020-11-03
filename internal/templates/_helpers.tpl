{{/*
check if we should create a kubernetes secret with docker credentials to pull images
*/}}
{{- define "dockerjson" -}}
{{- $username := .Values.dockerRegistry.username -}}
{{- $password := .Values.dockerRegistry.password -}}
{{- $name := .Values.dockerRegistry.registryName -}}
{{- $auth :=  (printf "%s:%s" $username $password) | b64enc -}}
{{- $value := (printf "{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"auth\":\"%v\"}}}\"" $name $username $password $auth ) -}}
{{- printf "%v" ( $value | b64enc ) -}}
{{- end -}}