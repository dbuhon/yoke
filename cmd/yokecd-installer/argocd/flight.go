package argocd

import (
	_ "embed"
	"fmt"

	"github.com/davidmdm/yoke/pkg/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//go:embed argo-cd-6.7.12.tgz
var archive []byte

// RenderChart renders the chart downloaded from https://argoproj.github.io/argo-helm/argo-cd
// Producing version: 6.7.12
func RenderChart(release, namespace string, values map[string]any) ([]*unstructured.Unstructured, error) {
	chart, err := helm.LoadChartFromZippedArchive(archive)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart from zipped archive: %w", err)
	}

	return chart.Render(release, namespace, values)
}
