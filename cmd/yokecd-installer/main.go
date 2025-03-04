package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/davidmdm/yoke/cmd/yokecd-installer/argocd"
	"golang.org/x/term"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	release   = os.Args[0]
	namespace = os.Getenv("NAMESPACE")
)

func run() error {
	values := map[string]any{
		"redis-ha": map[string]any{
			"enabled": false,
		},
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if err := yaml.NewYAMLToJSONDecoder(os.Stdin).Decode(&values); err != nil && err != io.EOF {
			return fmt.Errorf("failed to decode values: %w", err)
		}
	}

	resources, err := argocd.RenderChart(release, namespace, values)
	if err != nil {
		return fmt.Errorf("failed to render argocd chart: %w", err)
	}

	repoServer, i := func() (*unstructured.Unstructured, int) {
		repoServerName := release + "-argocd-repo-server"
		for i, resource := range resources {
			if resource.GetName() == repoServerName && resource.GetKind() == "Deployment" {
				return resource, i
			}
		}
		return nil, -1
	}()
	if i == -1 {
		return fmt.Errorf("cannot patch argocd: failed to find argocd-repo-server deployment")
	}

	var deployment appsv1.Deployment
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(repoServer.UnstructuredContent(), &deployment); err != nil {
		return fmt.Errorf("failed to convert argocd-repo-server to typed deployment: %w", err)
	}

	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
		Name:            "yokecd",
		Command:         []string{"/var/run/argocd/argocd-cmp-server"},
		Image:           "davidmdm/yokecd:latest",
		ImagePullPolicy: corev1.PullAlways,

		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "var-files",
				MountPath: "/var/run/argocd",
			},
			{
				Name:      "plugins",
				MountPath: "/home/argocd/cmp-server/plugins",
			},
			{
				Name:      "cmp-tmp",
				MountPath: "/tmp",
			},
		},

		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: ptr(true),
			RunAsUser:    ptr[int64](999),
		},
	})

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "cmp-tmp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	data, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	var resource *unstructured.Unstructured
	if err := json.Unmarshal(data, &resource); err != nil {
		return err
	}

	resources[i] = resource

	slices.SortFunc(resources, func(a, b *unstructured.Unstructured) int {
		const rbac = "rbac.authorization.k8s.io"
		groupA := a.GroupVersionKind().Group
		groupB := b.GroupVersionKind().Group

		switch {
		case groupA == "" && a.GetKind() == "ServiceAccount":
			return -1
		case groupB == "" && b.GetKind() == "ServiceAccount":
			return 1
		case groupA == rbac && groupB != rbac:
			return -1
		case groupA != rbac && groupB == rbac:
			return 1
		default:
			return 0
		}
	})

	var finalResources []*unstructured.Unstructured
	for _, resource := range resources {
		if strings.HasSuffix(resource.GetName(), "-test") {
			continue
		}
		finalResources = append(finalResources, resource)
	}

	return json.NewEncoder(os.Stdout).Encode(finalResources)
}

func ptr[T any](value T) *T { return &value }
