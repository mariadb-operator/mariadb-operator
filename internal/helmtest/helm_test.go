package test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	helmReleaseName = "mariadb-operator"
	helmChartPath   = "../../deploy/charts/mariadb-operator"
	testNamespace   = "test"
)

func TestHelmExtraEnvFrom(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		// see https://helm.sh/docs/intro/using_helm/#the-format-and-limitations-of---set
		SetValues: map[string]string{
			"extraEnvFrom[0].configMapRef.name": `env-extra-configmap`,
			"extraEnvFrom[1].secretRef.name":    `env-extra-secret`,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts, helmChartPath, helmReleaseName, []string{"templates/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	configMapEnvFrom := corev1.EnvFromSource{
		ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "env-extra-configmap",
			},
		},
	}
	Expect(container.EnvFrom).To(ContainElement(configMapEnvFrom))

	secretEnvFrom := corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "env-extra-secret",
			},
		},
	}
	Expect(container.EnvFrom).To(ContainElement(secretEnvFrom))
}

func TestHelmCurrentNamespaceOnly(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"currentNamespaceOnly": `true`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: testNamespace,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts, helmChartPath, helmReleaseName, []string{"templates/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	env := corev1.EnvVar{
		Name:  "WATCH_NAMESPACE",
		Value: testNamespace,
	}
	Expect(container.Env).To(ContainElement(env))

	expectedTemplates := []string{
		"templates/rbac-namespace.yaml",
	}
	unexpectedTemplates := []string{
		"templates/cert-controller-deployment.yaml",
		"templates/cert-controller-rbac.yaml",
		"templates/cert-controller-serviceaccount.yaml",
		"templates/cert-controller-servicemonitor.yaml",
		"templates/rbac-user.yaml",
		"templates/rbac.yaml",
		"templates/webhook-certificate.yaml",
		"templates/webhook-config.yaml",
		"templates/webhook-deployment.yaml",
		"templates/webhook-secret.yaml",
		"templates/webhook-service.yaml",
		"templates/webhook-serviceaccount.yaml",
		"templates/webhook-servicemonitor.yaml",
	}
	testHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestHelmClusterWide(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"webhook.cert.certManager.enabled": `true`,
			"metrics.enabled":                  `true`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: testNamespace,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts, helmChartPath, helmReleaseName, []string{"templates/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	env := corev1.EnvVar{
		Name:  "WATCH_NAMESPACE",
		Value: testNamespace,
	}
	Expect(container.Env).ToNot(ContainElement(env))

	expectedTemplates := []string{
		"templates/rbac-user.yaml",
		"templates/rbac.yaml",
		"templates/webhook-certificate.yaml",
		"templates/webhook-config.yaml",
		"templates/webhook-deployment.yaml",
		"templates/webhook-secret.yaml",
		"templates/webhook-service.yaml",
		"templates/webhook-serviceaccount.yaml",
	}
	unexpectedTemplates := []string{
		"templates/rbac-namespace.yaml",
	}
	testHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestHelmCertManager(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"webhook.cert.certManager.enabled": `true`,
		},
	}

	expectedTemplates := []string{
		"templates/webhook-certificate.yaml",
		"templates/webhook-secret.yaml",
	}
	unexpectedTemplates := []string{
		"templates/cert-controller-deployment.yaml",
		"templates/cert-controller-rbac.yaml",
		"templates/cert-controller-serviceaccount.yaml",
	}
	testHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestHelmMetrics(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"metrics.enabled": `true`,
		},
	}

	expectedTemplates := []string{
		"templates/cert-controller-servicemonitor.yaml",
		"templates/webhook-servicemonitor.yaml",
	}
	testHelmTemplates(t, opts, expectedTemplates, nil)
}

func testHelmTemplates(t *testing.T, opts *helm.Options, expectedTemplates, unexpectedTemplates []string) {
	for _, tpl := range expectedTemplates {
		_, err := helm.RenderTemplateE(t, opts, helmChartPath, helmReleaseName, []string{tpl})
		Expect(err).ToNot(HaveOccurred())
	}

	for _, tpl := range unexpectedTemplates {
		_, err := helm.RenderTemplateE(t, opts, helmChartPath, helmReleaseName, []string{tpl})
		Expect(err).To(HaveOccurred())
	}
}
