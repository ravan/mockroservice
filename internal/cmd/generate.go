package cmd

import (
	"bytes"
	"fmt"
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/template"
	"github.com/urfave/cli/v2"
	"io"
	"log/slog"
	"os"
	"strings"
)

func NewGenerateCommand() *cli.Command {
	return &cli.Command{
		Name:    "generate",
		Aliases: []string{"a"},
		Usage:   "Generate Helm Chart for configuration.",
		Action: func(ctx *cli.Context) error {
			configFile := ctx.String("config")
			if configFile == "" {
				return cli.Exit("config file is required", 1)
			}
			name := ctx.String("name")
			output := ctx.String("output")
			slog.Info("Generating Helm Chart ", "name", name)
			err := processMultipartConfig(configFile, name, output)
			if err != nil {
				return err
			}
			slog.Info("🎉 Done!")
			return nil
		},
		Flags: []cli.Flag{&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "target",
			Usage:   "Output directory",
		},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Value:   "sim-service",
				Usage:   "Chart name",
			}},
	}
}

type ConfigData struct {
	Content []byte
	Config  *config.Configuration
}

func processMultipartConfig(fileName, chartName, outputDir string) error {
	configs, err := getConfigs(fileName)
	if err != nil {
		return err
	}

	chartDir := mustMkdirAll(appendPath(outputDir, chartName))
	templateDir := mustMkdirAll(appendPath(chartDir, "templates"))
	dataMap := template.DataMap{
		"chartName": chartName,
	}
	template.MustRenderToFile(chartYamlTemplate, appendPath(chartDir, "Chart.yaml"), dataMap)
	template.MustRenderToFile(appReadMeTemplate, appendPath(chartDir, "app-readme.md"), dataMap)
	template.MustRenderToFile(questionsTemplate, appendPath(chartDir, "questions.yaml"), dataMap)
	template.MustRenderToFile(valuesTemplate, appendPath(chartDir, "values.yaml"), dataMap)

	template.MustRenderToFile(helpersTplTemplate, appendPath(templateDir, "_helpers.tpl"), dataMap)

	target := func(service, suffix string) string {
		return appendPath(templateDir, fmt.Sprintf("%s-%s.yaml", service, suffix))
	}
	for _, service := range configs {
		serviceName := sanitizeName(service.Config.ServiceName)
		dataFunc := getConfigDataFunc(service)
		template.MustRenderToFile(configMapTemplate, target(serviceName, "cm"), dataFunc)
		template.MustRenderToFile(deploymentTemplate, target(serviceName, "deployment"), dataFunc)
		template.MustRenderToFile(serviceTemplate, target(serviceName, "svc"), dataFunc)
	}

	return nil

}

func sanitizeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

func getConfigDataFunc(config *ConfigData) template.DataFunc {
	return template.DataFunc{
		TagFunc: func(w io.Writer, tag string) (int, error) {
			switch tag {
			case "serviceName":
				return w.Write([]byte(sanitizeName(config.Config.ServiceName)))

			case "config":
				indentContent := bytes.ReplaceAll(config.Content, []byte("\n"), []byte("\n     "))
				return w.Write(indentContent)

			default:
				return w.Write([]byte(fmt.Sprintf("[unknown tag %q]", tag)))
			}
		},
	}
}

func appendPath(src, file string) string {
	return fmt.Sprintf("%s/%s", src, file)
}

func mustMkdirAll(path string) string {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return path
}

func getConfigs(fileName string) ([]*ConfigData, error) {
	_, err := os.Stat(fileName)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	separator := []byte("+++")
	tomls := bytes.Split(content, separator)
	if len(tomls) == 0 {
		return nil, fmt.Errorf("no configuration found in %s", fileName)
	}

	var configs []*ConfigData
	for _, t := range tomls {
		temp, err := createTempFile(t)
		if err != nil {
			return nil, err
		}
		configuration, err := config.GetConfig(temp)
		_ = os.Remove(temp)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &ConfigData{
			Content: t,
			Config:  configuration,
		})

	}
	return configs, nil
}

func createTempFile(t []byte) (string, error) {
	temp, err := os.CreateTemp("", "sim-service")
	if err != nil {
		return "", err
	}
	if _, err = temp.Write(t); err != nil {
		return "", err
	}
	err = temp.Close()
	if err != nil {
		return "", err
	}
	return temp.Name(), nil
}

const chartYamlTemplate = `apiVersion: v2
name: [[chartName]] 
description: A Helm chart for [[chartName]] Mackroservices
type: application
version: 0.1.0
appVersion: "0.1.0"
keywords:
- challenge
- observability
`

const appReadMeTemplate = `## Introduction

Write description for Rancher App '[[Chart.Name]]'
`

const questionsTemplate = `questions:
- variable: hungry
  label: "Hungry"
  type: string
  default: "no"
  description: "Is the cookie monster hungry?"
  required: true
  group: General
`

const valuesTemplate = `hungry: 'no'
nameOverride: ''
fullnameOverride: ''
otelHttpEndpoint: opentelemetry-collector.open-telemetry.svc.cluster.local:4318
traceEnabled: false
metricsEnabled: false
image: ravan/mockroservice:latest
resources:
  requests:
    memory: '8Mi'
    cpu: '5m'
  limits:
    memory: '10Mi'
    cpu: '10m'
`

const helpersTplTemplate = `{{/*
Expand the name of the chart.
*/}}
{{- define "common.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "common.fullname" -}}
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
{{- define "common.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "common.labels" -}}
helm.sh/chart: {{ include "common.chart" . }}
{{ include "common.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "common.selectorLabels" -}}
app.kubernetes.io/name: {{ include "common.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "common.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "common.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
`

const configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: [[serviceName]]-cm
  labels:
    {{- include "common.labels" . | nindent 4 }}
data:
  config.toml: |
     [[config]]
     
     [otel.trace]
     enabled = {{.Values.traceEnabled}}
     tracer-name = "[[serviceName]]"
     http-endpoint = "{{.Values.otelHttpEndpoint}}"
     insecure = true

     [otel.metrics]
     enabled = {{.Values.metricsEnabled}}
     http-endpoint = "{{.Values.otelHttpEndpoint}}" 
     insecure = true
`

const deploymentTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: [[serviceName]]
  labels:
    service: [[serviceName]]
    {{- include "common.labels" . | nindent 4 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      service: [[serviceName]]
      {{- include "common.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "common.labels" . | nindent 8 }}
        service: [[serviceName]] 
      annotations:
        checksum/config: '{{ include (print $.Template.BasePath "/[[serviceName]]-cm.yaml") . | sha256sum}}'
    spec:
      containers:
      - name: [[serviceName]]
        image: {{.Values.image}}
        env:
        - name: CONFIG_FILE
          value: /etc/app/config.toml
        ports:
        - containerPort: 8080
        resources:
          {{- toYaml .Values.resources | nindent 12 }} 
        volumeMounts:
        - name: config-volume
          mountPath: /etc/app
      volumes:
      - name: config-volume
        configMap:
          name: [[serviceName]]-cm
          items:
          - key: config.toml
            path: config.toml
`

const serviceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: [[serviceName]]
  labels:
    service: [[serviceName]]
    {{- include "common.labels" . | nindent 4 }}
spec:
  selector:
    service: [[serviceName]]
    {{- include "common.selectorLabels" . | nindent 4 }}
  ports:
    - protocol: TCP
      port: 80      # Service port
      targetPort: 8080 # Container port
  type: ClusterIP     # Internal service within the Kubernetes cluster
`
