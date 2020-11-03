package templates

type YamlFile struct {
	Name    string
	Content string
}

var (
	DefaultYamls = map[string]string{
		"_helpers.tpl": `{{/*
check if we should create a kubernetes secret with docker credentials to pull images
*/}}
{{- define "dockerjson" -}}
{{- $username := .Values.dockerRegistry.username -}}
{{- $password := .Values.dockerRegistry.password -}}
{{- $name := .Values.dockerRegistry.registryName -}}
{{- $auth :=  (printf "%s:%s" $username $password) | b64enc -}}
{{- $value := (printf "{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"auth\":\"%v\"}}}\"" $name $username $password $auth ) -}}
{{- printf "%v" ( $value | b64enc ) -}}
{{- end -}}`,
		"deployment.yaml": `{{ range $_, $deployment := .Values.app.deployments }}
  {{ range $_, $process := $deployment.processes }}
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: {{ printf "%s-%s-%v" $.Values.app.name $process.name $deployment.version }}
    theketch.io/app-name: {{ $.Values.app.name }}
    theketch.io/app-process: {{ $process.name }}
    theketch.io/app-process-replicas: {{ $process.units | quote }}
    theketch.io/app-deployment-version: {{ $deployment.version | quote }}
    theketch.io/is-isolated-run: "false"
    {{- range $i, $label := $deployment.labels }}
    {{ $label.name }}: {{ $label.value }}
    {{- end }}
  name: {{ $.Values.app.name }}-{{ $process.name }}-{{ $deployment.version }}
spec:
  replicas: {{ $process.units }}
  selector:
    matchLabels:
      app: {{ printf "%s-%s-%v" $.Values.app.name $process.name $deployment.version }}
      theketch.io/app-name: {{ $.Values.app.name }}
      theketch.io/app-process: {{ $process.name }}
      theketch.io/app-deployment-version: {{ $deployment.version | quote }}
      theketch.io/is-isolated-run: "false"
  template:
    metadata:
      labels:
        app: {{ printf "%s-%s-%v" $.Values.app.name $process.name $deployment.version }}
        theketch.io/app-name: {{ $.Values.app.name }}
        theketch.io/app-process: {{ $process.name }}
        theketch.io/app-deployment-version: {{ $deployment.version | quote }}
        theketch.io/is-isolated-run: "false"
    spec:
      containers:
        - name: {{ $.Values.app.name }}-{{ $process.name }}-{{ $deployment.version }}
          command: {{ $process.cmd | toJson }}
          {{- if or $process.env $.Values.app.env }}
          env:
          {{- if $process.env }}
{{ $process.env | toYaml | indent 12 }}
          {{- end }}
          {{- if $.Values.app.env }}
{{ $.Values.app.env | toYaml | indent 12 }}
          {{- end }}
          {{- end }}
          image: {{ $deployment.image }}
          {{- if $process.containerPorts }}
          ports:
{{ $process.containerPorts | toYaml | indent 10 }}
          {{- end }}
          {{- if $process.extra.volumeMounts }}
          volumeMounts:
{{ $process.extra.volumeMounts | toYaml | indent 12 }}
          {{- end }}
          {{- if $process.extra.resourceRequirements }}
          resources:
{{ $process.extra.resourceRequirements | toYaml | indent 12 }}
          {{- end }}
          {{- if $process.extra.lifecycle }}
          lifecycle:
{{ $process.extra.lifecycle | toYaml | indent 12 }}
          {{- end }}
          {{- if $process.extra.securityContext }}
          securityContext:
{{ $process.extra.securityContext | toYaml | indent 12 }}
          {{- end }}
      {{- if or $.Values.dockerRegistry.imagePullSecret $.Values.dockerRegistry.createImagePullSecret }}
      imagePullSecrets:
      {{- if $.Values.dockerRegistry.imagePullSecret }}
        - name: {{ $.Values.dockerRegistry.imagePullSecret }}
      {{- end }}
      {{- if $.Values.dockerRegistry.createImagePullSecret }}
        - name: {{ $.Values.app.name }}-image-pull-secret
      {{- end }}
      {{- end }}
      {{- if $deployment.extra.volumes }}
      volumes:
{{ $deployment.extra.volumes | toYaml | indent 12 }}
      {{- end }}
      {{- if $process.extra.nodeSelectorTerms }}
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
{{ $process.extra.nodeSelectorTerms | toYaml | indent 14 }}
      {{- end }}
---
{{ end }}
{{ end }}
`,
		"image-pull-secret.yaml": `{{- if .Values.dockerRegistry.createImagePullSecret }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Values.app.name }}-image-pull-secret
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: {{ template "dockerjson" . }}
{{- end }}`,
		"ingress.yaml": `{{- if .Values.app.deployments }}
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  annotations:
    {{- if .Values.ingressController.className }}
    kubernetes.io/ingress.class: {{ .Values.ingressController.className }}
    {{- end }}
    {{- if .Values.ingressController.traefik }}
    traefik.ingress.kubernetes.io/frontend-entry-points: {{ join "," .Values.ingressController.traefik.entryPoints }}
    traefik.ingress.kubernetes.io/service-weights: |
	  {{- range $_, $deployment := .Values.app.deployments }}
      {{- range $_, $process := $deployment.processes }}
      {{- if $process.routable }}
      {{ printf "%s-%s-%v" $.Values.app.name $process.name $deployment.version }}: {{ $deployment.routingSettings.weight }}
      {{- end }}
      {{- end }}
      {{- end }}
    {{- end }}
  labels:
    theketch.io/app-name: {{ $.Values.app.name }}
  name: {{ $.Values.app.name }}-http
spec:
  rules:
    {{- range $_, $cname := .Values.app.cnames }}
    - host: {{ $cname }}
      http:
        paths:
          {{- range $_, $deployment := $.Values.app.deployments }}
          {{- range $_, $process := $deployment.processes }}
          {{- if $process.routable }}
          - backend:
              serviceName: {{ printf "%s-%s-%v" $.Values.app.name $process.name $deployment.version }}
              servicePort: {{ $process.publicServicePort }}
          {{- end }}
          {{- end }}
          {{- end }}
    {{- end }}
{{- end }}`,
		"service.yaml": `{{ range $_, $deployment := .Values.app.deployments }}
  {{ range $_, $process := $deployment.processes }}
  {{- if $process.servicePorts }}
apiVersion: v1
kind: Service
metadata:
  labels:
    app: {{ printf "%s-%s-%v" $.Values.app.name $process.name $deployment.version }}
    theketch.io/app-name: {{ $.Values.app.name }}
    theketch.io/app-process: {{ $process.name }}
    theketch.io/app-deployment-version: {{ $deployment.version | quote }}
    theketch.io/is-isolated-run: "false"
    {{- range $i, $label := $deployment.labels }}
    {{ $label.name }}: {{ $label.value }}
    {{- end }}
  name: {{ $.Values.app.name }}-{{ $process.name }}-{{ $deployment.version }}
spec:
  type: ClusterIP
  ports:
{{ $process.servicePorts | toYaml | indent 4 }}
  selector:
    theketch.io/app-name: {{ $.Values.app.name }}
    theketch.io/app-process: {{ $process.name }}
    theketch.io/app-deployment-version: {{ $deployment.version | quote }}
    theketch.io/is-isolated-run: "false"
---
  {{- end }}
  {{ end }}
{{ end }}
`,
	}
)
