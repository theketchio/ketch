{{/*

ketch.renderMetadata renders a labels/annotations section based on a given dict,
the dict must have the following entries:
{
    "metadataItems": []MetadataItem{},    // a list of requests to add metadata
    "kind": "<kind>",                   // all metadataItems with target.kind equals <kind> will be added
    "apiVersion": "<apiVersion>",       // all metadataItems with target.apiVersion equals <kind> will be added
}

This is an example of usage:
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
    {{- $data := dict "kind" "Gateway" "apiVersion" "networking.istio.io/v1alpha3" "metadataItems" $.Values.app.metadataAnnotations }}
    annotations: {{- include "ketch.renderMetadata" $data | nindent 4 }}

Two theketch.io annotations are added to simplify debug and to avoid dealing with an empty "labels/annotations" section in yamls.

*/}}
{{- define "ketch.renderMetadata" -}}
theketch.io/metadata-item-kind: {{ $.kind }}
theketch.io/metadata-item-apiVersion: {{ $.apiVersion }}
{{- range $_, $item := $.metadataItems }}
  {{- if eq $item.target.kind $.kind }}
    {{- if eq $item.target.apiVersion $.apiVersion }}
        {{- range $key, $value := $item.apply }}
{{ $key }}: {{ $value | quote }}
        {{- end }}
    {{- end }}
{{- end }}
{{- end }}
{{- end -}}
