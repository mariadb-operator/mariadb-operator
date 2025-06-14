package test

import (
	"strconv"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	clusterHelmReleaseName = "mariadb-cluster"
	clusterHelmChartPath   = "../../deploy/charts/mariadb-cluster"
	clusterHelmNamespace   = "default"
)

func TestClusterHelmMariaDB(t *testing.T) {
	RegisterTestingT(t)

	replicas := 3
	storageSize := "1Gi"
	rootPasswordSecretKeyRefKey := "ROOT_PASSWORD"
	rootPasswordSecretKeyRefName := "mariadb-cluster-passwords"

	opts := &helm.Options{
		SetValues: map[string]string{
			"mariadb.galera.enabled":                "true",
			"mariadb.metrics.enabled":               "true",
			"mariadb.replicas":                      strconv.Itoa(replicas),
			"mariadb.storage.size":                  storageSize,
			"mariadb.rootPasswordSecretKeyRef.key":  rootPasswordSecretKeyRefKey,
			"mariadb.rootPasswordSecretKeyRef.name": rootPasswordSecretKeyRefName,
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/mariadb.yaml"})
	var mariadb v1alpha1.MariaDB
	helm.UnmarshalK8SYaml(t, renderedData, &mariadb)

	Expect(mariadb.Spec.Galera.Enabled).To(BeTrue())
	Expect(mariadb.Spec.Metrics.Enabled).To(BeTrue())
	Expect(mariadb.Spec.Replicas).To(Equal(int32(replicas)))
	Expect(mariadb.Spec.Storage.Size.String()).To(Equal(storageSize))
	Expect(mariadb.Spec.RootPasswordSecretKeyRef.Key).To(Equal(rootPasswordSecretKeyRefKey))
	Expect(mariadb.Spec.RootPasswordSecretKeyRef.Name).To(Equal(rootPasswordSecretKeyRefName))
}

func TestClusterHelmDatabase(t *testing.T) {
	RegisterTestingT(t)

	characterSet := "utf8"
	cleanupPolicy := "Delete"
	collate := "utf8_general_ci"
	name := "foo"
	requeueInterval := "1h"
	retryInterval := "3m"

	durationInterval, _ := time.ParseDuration(requeueInterval)
	durationRetry, _ := time.ParseDuration(retryInterval)
	requeueIntervalDuration := &v1.Duration{Duration: durationInterval}
	retryIntervalDuration := &v1.Duration{Duration: durationRetry}

	opts := &helm.Options{
		SetValues: map[string]string{
			"databases[0].characterSet":         characterSet,
			"databases[0].cleanupPolicy":        cleanupPolicy,
			"databases[0].collate":              collate,
			"databases[0].name":                 name,
			"databases[0].requeueInterval":      requeueInterval,
			"databases[0].retryInterval":        retryInterval,
			"databases[0].mariaDBRef.name":      "cluster",
			"databases[0].mariaDBRef.namespace": "namaspace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/database.yaml"})
	var database v1alpha1.Database
	helm.UnmarshalK8SYaml(t, renderedData, &database)

	Expect(database.Spec.MariaDBRef.Name).To(Equal(clusterHelmReleaseName))
	Expect(database.Spec.MariaDBRef.Namespace).To(Equal(clusterHelmNamespace))
	Expect(database.Spec.CharacterSet).To(Equal(characterSet))
	Expect(*database.Spec.CleanupPolicy).To(Equal(v1alpha1.CleanupPolicy(cleanupPolicy)))
	Expect(database.Spec.Collate).To(Equal(collate))
	Expect(database.Spec.Name).To(Equal(name))
	Expect(database.Spec.RequeueInterval).To(Equal(requeueIntervalDuration))
	Expect(database.Spec.RetryInterval).To(Equal(retryIntervalDuration))

	delete(opts.SetValues, "databases[0].name")
	_, err := helm.RenderTemplateE(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/database.yaml"})
	Expect(err).To(HaveOccurred())
}

func TestClusterHelmUser(t *testing.T) {
	RegisterTestingT(t)

	cleanupPolicy := "Delete"
	host := "%"
	maxUserConnections := 100
	name := "foo"
	passwordSecretKeyRefKey := "FOO_PASSWORD"
	passwordSecretKeyRefName := "mariadb-cluster-passwords"
	requeueInterval := "1h"
	retryInterval := "3m"

	durationInterval, _ := time.ParseDuration(requeueInterval)
	durationRetry, _ := time.ParseDuration(retryInterval)
	requeueIntervalDuration := &v1.Duration{Duration: durationInterval}
	retryIntervalDuration := &v1.Duration{Duration: durationRetry}

	opts := &helm.Options{
		SetValues: map[string]string{
			"users[0].cleanupPolicy":             cleanupPolicy,
			"users[0].host":                      host,
			"users[0].maxUserConnections":        strconv.Itoa(maxUserConnections),
			"users[0].name":                      name,
			"users[0].passwordSecretKeyRef.key":  passwordSecretKeyRefKey,
			"users[0].passwordSecretKeyRef.name": passwordSecretKeyRefName,
			"users[0].requeueInterval":           requeueInterval,
			"users[0].retryInterval":             retryInterval,
			"users[0].mariaDBRef.name":           "cluster",
			"users[0].mariaDBRef.namespace":      "namaspace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/user.yaml"})
	var user v1alpha1.User
	helm.UnmarshalK8SYaml(t, renderedData, &user)

	Expect(user.Spec.MariaDBRef.Name).To(Equal(clusterHelmReleaseName))
	Expect(user.Spec.MariaDBRef.Namespace).To(Equal(clusterHelmNamespace))
	Expect(*user.Spec.CleanupPolicy).To(Equal(v1alpha1.CleanupPolicy(cleanupPolicy)))
	Expect(user.Spec.Host).To(Equal(host))
	Expect(user.Spec.MaxUserConnections).To(Equal(int32(maxUserConnections)))
	Expect(user.Spec.Name).To(Equal(name))
	Expect(user.Spec.PasswordSecretKeyRef.Key).To(Equal(passwordSecretKeyRefKey))
	Expect(user.Spec.PasswordSecretKeyRef.Name).To(Equal(passwordSecretKeyRefName))
	Expect(user.Spec.RequeueInterval).To(Equal(requeueIntervalDuration))
	Expect(user.Spec.RetryInterval).To(Equal(retryIntervalDuration))

	delete(opts.SetValues, "users[0].name")
	_, err := helm.RenderTemplateE(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/user.yaml"})
	Expect(err).To(HaveOccurred())
}

func TestClusterHelmGrant(t *testing.T) {
	RegisterTestingT(t)

	cleanupPolicy := "Delete"
	database := "foo"
	host := "%"
	username := "foo"
	privilegeSelect := "SELECT"
	privilegeProcess := "PROCESS"
	requeueInterval := "1h"
	retryInterval := "3m"
	table := "*"

	durationInterval, _ := time.ParseDuration(requeueInterval)
	durationRetry, _ := time.ParseDuration(retryInterval)
	requeueIntervalDuration := &v1.Duration{Duration: durationInterval}
	retryIntervalDuration := &v1.Duration{Duration: durationRetry}

	opts := &helm.Options{
		SetValues: map[string]string{
			"grants[0].cleanupPolicy":        cleanupPolicy,
			"grants[0].database":             database,
			"grants[0].host":                 host,
			"grants[0].grantOption":          "false",
			"grants[0].username":             username,
			"grants[0].privileges[0]":        privilegeSelect,
			"grants[0].privileges[1]":        privilegeProcess,
			"grants[0].requeueInterval":      requeueInterval,
			"grants[0].retryInterval":        retryInterval,
			"grants[0].table":                table,
			"grants[0].mariaDBRef.name":      "cluster",
			"grants[0].mariaDBRef.namespace": "namaspace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/grant.yaml"})
	var grant v1alpha1.Grant
	helm.UnmarshalK8SYaml(t, renderedData, &grant)

	Expect(grant.Spec.MariaDBRef.Name).To(Equal(clusterHelmReleaseName))
	Expect(grant.Spec.MariaDBRef.Namespace).To(Equal(clusterHelmNamespace))
	Expect(*grant.Spec.CleanupPolicy).To(Equal(v1alpha1.CleanupPolicy(cleanupPolicy)))
	Expect(grant.Spec.Database).To(Equal(database))
	Expect(*grant.Spec.Host).To(Equal(host))
	Expect(grant.Spec.GrantOption).To(BeFalse())
	Expect(grant.Spec.Username).To(Equal(username))
	Expect(grant.Spec.Privileges[0]).To(Equal(privilegeSelect))
	Expect(grant.Spec.Privileges[1]).To(Equal(privilegeProcess))
	Expect(grant.Spec.RequeueInterval).To(Equal(requeueIntervalDuration))
	Expect(grant.Spec.RetryInterval).To(Equal(retryIntervalDuration))
	Expect(grant.Spec.Table).To(Equal(table))

	delete(opts.SetValues, "grants[0].username")
	_, err := helm.RenderTemplateE(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/grant.yaml"})
	Expect(err).To(HaveOccurred())
}
