{{/* Generate pod template for deployment and stateful_set */}}
{{- define "app.podTemplate" }}
    spec:
      {{- if .root.app.serviceAccountName }}
      serviceAccountName: {{ .root.app.serviceAccountName }}
      {{- end }}
      {{- if .root.app.securityContext }}
      securityContext:
{{ .root.app.securityContext | toYaml | indent 8 }}
      {{- end }}
      containers:
        - name: {{ .root.app.name }}-{{ .process.name }}-{{ .deployment.version }}
          command: {{ .process.cmd | toJson }}
          {{- if or .process.env .root.app.env }}
          env:
          {{- if .process.env }}
{{ .process.env | toYaml | indent 12 }}
          {{- end }}
          {{- if .root.app.env }}
{{ .root.app.env | toYaml | indent 12 }}
          {{- end }}
          {{- end }}
          image: {{ .deployment.image }}
          {{- if .process.containerPorts }}
          ports:
{{ .process.containerPorts | toYaml | indent 10 }}
          {{- end }}
          {{- if .process.volumeMounts }}
          volumeMounts:
{{ .process.volumeMounts | toYaml | indent 12 }}
          {{- end }}
          {{- if .process.resourceRequirements }}
          resources:
{{ .process.resourceRequirements | toYaml | indent 12 }}
          {{- end }}
          {{- if .process.lifecycle }}
          lifecycle:
{{ .process.lifecycle | toYaml | indent 12 }}
          {{- end }}
          {{- if .process.securityContext }}
          securityContext:
{{ .process.securityContext | toYaml | indent 12 }}
          {{- end }}
          {{- if .process.readinessProbe }}
          readinessProbe:
{{ .process.readinessProbe | toYaml | indent 12 }}
          {{- end }}
          {{- if .process.livenessProbe }}
          livenessProbe:
{{ .process.livenessProbe | toYaml | indent 12 }}
          {{- end }}
          {{- if .process.startupProbe }}
          startupProbe:
{{ .process.startupProbe | toYaml | indent 12 }}
          {{- end }}
      {{- if .deployment.imagePullSecrets }}
      imagePullSecrets:
{{ .deployment.imagePullSecrets | toYaml | indent 12}}
      {{- end }}
      {{- if .process.volumes }}
      volumes:
{{ .process.volumes | toYaml | indent 12 }}
      {{- end }}
      {{- if .process.nodeSelectorTerms }}
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
{{ .process.nodeSelectorTerms | toYaml | indent 14 }}
      {{- end }}
{{- end }}