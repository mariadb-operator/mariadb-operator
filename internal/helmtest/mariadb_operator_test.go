package test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

var (
	operatorHelmReleaseName = "mariadb-operator"
	operatorHelmChartPath   = "../../deploy/charts/mariadb-operator"
	operatorTestNamespace   = "test"
)

func TestOperatorHelmExtraEnvFrom(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		// see https://helm.sh/docs/intro/using_helm/#the-format-and-limitations-of---set
		SetValues: map[string]string{
			"extraEnvFrom[0].configMapRef.name": `env-extra-configmap`,
			"extraEnvFrom[1].secretRef.name":    `env-extra-secret`,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/deployment.yaml"})
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

func TestOperatorHelmCurrentNamespaceOnly(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"currentNamespaceOnly": `true`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	env := corev1.EnvVar{
		Name:  "WATCH_NAMESPACE",
		Value: operatorTestNamespace,
	}
	Expect(container.Env).To(ContainElement(env))

	expectedTemplates := []string{
		"templates/operator/rbac-namespace.yaml",
	}
	unexpectedTemplates := []string{
		"templates/cert-controller/deployment.yaml",
		"templates/cert-controller/rbac.yaml",
		"templates/cert-controller/serviceaccount.yaml",
		"templates/cert-controller/servicemonitor.yaml",
		"templates/operator/rbac-user.yaml",
		"templates/operator/rbac.yaml",
		"templates/webhook/certificate.yaml",
		"templates/webhook/config.yaml",
		"templates/webhook/deployment.yaml",
		"templates/webhook/service.yaml",
		"templates/webhook/serviceaccount.yaml",
		"templates/webhook/servicemonitor.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestOperatorHelmClusterWide(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"webhook.cert.certManager.enabled": `true`,
			"metrics.enabled":                  `true`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	env := corev1.EnvVar{
		Name:  "WATCH_NAMESPACE",
		Value: operatorTestNamespace,
	}
	Expect(container.Env).ToNot(ContainElement(env))

	expectedTemplates := []string{
		"templates/operator/rbac-user.yaml",
		"templates/operator/rbac.yaml",
		"templates/webhook/certificate.yaml",
		"templates/webhook/config.yaml",
		"templates/webhook/deployment.yaml",
		"templates/webhook/service.yaml",
		"templates/webhook/serviceaccount.yaml",
	}
	unexpectedTemplates := []string{
		"templates/operator/rbac-namespace.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestOperatorHelmCertManager(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"webhook.cert.certManager.enabled": `true`,
		},
	}

	expectedTemplates := []string{
		"templates/webhook/certificate.yaml",
	}
	unexpectedTemplates := []string{
		"templates/cert-controller/deployment.yaml",
		"templates/cert-controller/rbac.yaml",
		"templates/cert-controller/serviceaccount.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestOperatorHelmMetrics(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"metrics.enabled": `true`,
		},
	}

	expectedTemplates := []string{
		"templates/cert-controller/servicemonitor.yaml",
		"templates/webhook/servicemonitor.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, nil)
}

func TestOperatorHelmWebhook(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"webhook.enabled": `true`,
		},
	}
	expectedTemplates := []string{
		"templates/webhook/config.yaml",
		"templates/webhook/deployment.yaml",
		"templates/webhook/service.yaml",
		"templates/webhook/serviceaccount.yaml",
	}
	unexpectedTemplates := []string{}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)

	opts = &helm.Options{
		SetValues: map[string]string{
			"webhook.enabled": `false`,
		},
	}
	expectedTemplates = []string{}
	unexpectedTemplates = []string{
		"templates/webhook/config.yaml",
		"templates/webhook/deployment.yaml",
		"templates/webhook/service.yaml",
		"templates/webhook/serviceaccount.yaml",
		"templates/webhook/pdb.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestOperatorHelmCertController(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"certController.enabled": `true`,
		},
	}
	expectedTemplates := []string{
		"templates/cert-controller/deployment.yaml",
		"templates/cert-controller/rbac.yaml",
		"templates/cert-controller/serviceaccount.yaml",
	}
	unexpectedTemplates := []string{}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)

	opts = &helm.Options{
		SetValues: map[string]string{
			"certController.enabled": `false`,
		},
	}
	expectedTemplates = []string{}
	unexpectedTemplates = []string{
		"templates/cert-controller/deployment.yaml",
		"templates/cert-controller/rbac.yaml",
		"templates/cert-controller/serviceaccount.yaml",
		"templates/cert-controller/pdb.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func TestOperatorHelmHaEnabled(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"ha.enabled": `true`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))

	replicas := int(*deployment.Spec.Replicas)
	Expect(replicas).To(Equal(3))
	Expect(container.Args).To(ContainElement("--leader-elect"))
}

func TestOperatorHelmPDBEnabled(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"pdb.enabled": `true`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}
	pdbData := helm.RenderTemplate(t, opts, operatorHelmChartPath, operatorHelmReleaseName, []string{"templates/operator/pdb.yaml"})
	var pdb policyv1.PodDisruptionBudget
	helm.UnmarshalK8SYaml(t, pdbData, &pdb)
	maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
	Expect(maxUnavailable).To(Equal(1))

	opts = &helm.Options{
		SetValues: map[string]string{
			"pdb.enabled":        `true`,
			"pdb.maxUnavailable": "50%",
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}
	pdbData = helm.RenderTemplate(t, opts, operatorHelmChartPath, operatorHelmReleaseName, []string{"templates/operator/pdb.yaml"})
	helm.UnmarshalK8SYaml(t, pdbData, &pdb)
	maxUnavailablePercent := pdb.Spec.MaxUnavailable.String()
	Expect(maxUnavailablePercent).To(Equal("50%"))

	expectedTemplates := []string{
		"templates/operator/pdb.yaml",
	}
	unexpectedTemplates := []string{}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)

	opts = &helm.Options{
		SetValues: map[string]string{
			"pdb.enabled": `false`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}
	expectedTemplates = []string{}
	unexpectedTemplates = []string{
		"templates/operator/pdb.yaml",
	}
	testOperatorHelmTemplates(t, opts, expectedTemplates, unexpectedTemplates)
}

func testOperatorHelmTemplates(t *testing.T, opts *helm.Options, expectedTemplates, unexpectedTemplates []string) {
	for _, tpl := range expectedTemplates {
		_, err := helm.RenderTemplateE(t, opts, operatorHelmChartPath, operatorHelmReleaseName, []string{tpl})
		Expect(err).ToNot(HaveOccurred())
	}

	for _, tpl := range unexpectedTemplates {
		_, err := helm.RenderTemplateE(t, opts, operatorHelmChartPath, operatorHelmReleaseName, []string{tpl})
		Expect(err).To(HaveOccurred())
	}
}

func TestOperatorHelmImageTagAndDigest(t *testing.T) {
	RegisterTestingT(t)

	repository := "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator"
	tag := "v1.0.0"
	digest := "sha256:abc123def456"

	opts := &helm.Options{
		SetValues: map[string]string{
			"image.repository": repository,
			"image.tag":        tag,
			"image.digest":     digest,
		},
	}

	renderedData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, renderedData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))
	Expect(container.Image).To(ContainSubstring(repository + "@" + digest))

	opts = &helm.Options{
		SetValues: map[string]string{
			"image.repository": repository,
			"image.tag":        tag,
		},
	}

	renderedData = helm.RenderTemplate(t, opts, operatorHelmChartPath, operatorHelmReleaseName, []string{"templates/operator/deployment.yaml"})
	helm.UnmarshalK8SYaml(t, renderedData, &deployment)

	container = deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))
	Expect(container.Image).To(ContainSubstring(repository + ":" + tag))
}

func TestOperatorHelmConfigMap(t *testing.T) {
	RegisterTestingT(t)
	repository := "docker-registry3.mariadb.com/mariadb-operator/mariadb-operator"
	tag := "v1.0.0"
	opts := &helm.Options{
		SetValues: map[string]string{
			"image.repository":             repository,
			"image.tag":                    tag,
			"config.galeraLibPath":         "/path/to/libgalera.so",
			"config.mariadbDefaultVersion": "11.4",
			"config.mariadbImage":          "mariadb:10.5",
			"config.maxscaleImage":         "maxscale:2.5",
			"config.exporterImage":         "exporter:1.0",
			"config.exporterMaxscaleImage": "exporter-maxscale:1.0",
		},
	}
	configMapData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/configmap.yaml"})
	var configMap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, configMapData, &configMap)
	Expect(configMap.Name).To(Equal("mariadb-operator-env"))
	Expect(configMap.Data["MARIADB_OPERATOR_IMAGE"]).To(Equal(repository + ":" + tag))
	Expect(configMap.Data["MARIADB_GALERA_LIB_PATH"]).To(Equal("/path/to/libgalera.so"))
	Expect(configMap.Data["MARIADB_DEFAULT_VERSION"]).To(Equal("11.4"))
	Expect(configMap.Data["RELATED_IMAGE_MARIADB"]).To(Equal("mariadb:10.5"))
	Expect(configMap.Data["RELATED_IMAGE_MAXSCALE"]).To(Equal("maxscale:2.5"))
	Expect(configMap.Data["RELATED_IMAGE_EXPORTER"]).To(Equal("exporter:1.0"))
	Expect(configMap.Data["RELATED_IMAGE_EXPORTER_MAXSCALE"]).To(Equal("exporter-maxscale:1.0"))
}

func TestOperatorHelmPprof(t *testing.T) {
	RegisterTestingT(t)
	opts := &helm.Options{
		SetValues: map[string]string{
			"pprof.enabled": `true`,
			"pprof.port":    `6060`,
		},
		KubectlOptions: &k8s.KubectlOptions{
			Namespace: operatorTestNamespace,
		},
	}

	deploymentData := helm.RenderTemplate(t, opts,
		operatorHelmChartPath, operatorHelmReleaseName,
		[]string{"templates/operator/deployment.yaml"})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, deploymentData, &deployment)

	container := deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal("controller"))
	Expect(container.Args).To(ContainElement("--pprof"))
	Expect(container.Args).To(ContainElement("--pprof-addr=:6060"))
	Expect(container.Ports).To(ContainElement(corev1.ContainerPort{
		Name:          "pprof",
		Protocol:      "TCP",
		ContainerPort: 6060,
	}))
}
