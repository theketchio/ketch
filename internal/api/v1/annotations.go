package v1

import "fmt"

// DontUninstallHelmChartAnnotation returns an annotation that prevents
// ketch-controller from uninstalling helm chart of Application or Job.
func DontUninstallHelmChartAnnotation(group string) string {
	return fmt.Sprintf("%s/dont-uninstall-helm-chart", group)
}
