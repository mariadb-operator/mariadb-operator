package test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/gomega"

	"github.com/gruntwork-io/terratest/modules/helm"
)

func TestHelmTemplatesIncludeExtraEnvFrom(t *testing.T) {

	helmChartPath := "../../deploy/charts/mariadb-operator"

	RegisterTestingT(t)
	options := &helm.Options{

		// see https://helm.sh/docs/intro/using_helm/#the-format-and-limitations-of---set
		SetValues: map[string]string{
			"extraEnvFrom[0].configMapRef.name": `env-extra-configmap`,
			"extraEnvFrom[1].secretRef.name":    `env-extra-secret`,
		},
	}

	// Run RenderTemplate to render but only `templates/deployment.yaml`
	output := helm.RenderTemplate(t, options, helmChartPath, "mariadb-operator", []string{"templates/deployment.yaml"})

	// Unmarshal for assertions
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, output, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	cmSource := corev1.EnvFromSource{}
	cmSource.ConfigMapRef = &corev1.ConfigMapEnvSource{}
	cmSource.ConfigMapRef.Name = "env-extra-configmap"
	Expect(container.EnvFrom).To(ContainElement(cmSource))

	secretSource := corev1.EnvFromSource{}
	secretSource.SecretRef = &corev1.SecretEnvSource{}
	secretSource.SecretRef.Name = "env-extra-secret"
	Expect(container.EnvFrom).To(ContainElement(secretSource))

}
