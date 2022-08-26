package controllers

import (
	"strconv"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
)

// uninstallHelmChart checks if there is a special annotation that
// prevents ketch-controller from uninstalling helm chart of App/Job.
func uninstallHelmChart(group string, annotations map[string]string) bool {
	if len(annotations) == 0 {
		return true
	}
	value, ok := annotations[ketchv1.DontUninstallHelmChartAnnotation(group)]
	if !ok {
		return true
	}
	keepChart, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return !keepChart
}
