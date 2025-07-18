package test

import (
	"fmt"
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
	rootPasswordSecretKeyRefKey := "mariadb"
	rootPasswordSecretKeyRefName := "root-password"

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

	name := "mariadb"
	namespace := "database"
	characterSet := "utf8"
	cleanupPolicy := "Delete"
	collate := "utf8_general_ci"
	requeueInterval := "10h"
	retryInterval := "30s"

	durationInterval, _ := time.ParseDuration(requeueInterval)
	durationRetry, _ := time.ParseDuration(retryInterval)
	requeueIntervalDuration := &v1.Duration{Duration: durationInterval}
	retryIntervalDuration := &v1.Duration{Duration: durationRetry}

	opts := &helm.Options{
		SetValues: map[string]string{
			"databases[0].name":                 name,
			"databases[0].namespace":            namespace,
			"databases[0].characterSet":         characterSet,
			"databases[0].cleanupPolicy":        cleanupPolicy,
			"databases[0].collate":              collate,
			"databases[0].requeueInterval":      requeueInterval,
			"databases[0].retryInterval":        retryInterval,
			"databases[0].mariaDBRef.name":      "cluster",
			"databases[0].mariaDBRef.namespace": "namespace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/database.yaml"})
	var database v1alpha1.Database
	helm.UnmarshalK8SYaml(t, renderedData, &database)

	Expect(database.Name).To(Equal(fmt.Sprintf("%s-%s", clusterHelmReleaseName, name)))
	Expect(database.Namespace).To(Equal(namespace))
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

	name := "mariadb"
	namespace := "database"
	cleanupPolicy := "Delete"
	host := "%"
	maxUserConnections := 100
	passwordSecretKeyRefKey := "mariadb"
	passwordSecretKeyRefName := "password"
	requeueInterval := "10h"
	retryInterval := "30s"

	durationInterval, _ := time.ParseDuration(requeueInterval)
	durationRetry, _ := time.ParseDuration(retryInterval)
	requeueIntervalDuration := &v1.Duration{Duration: durationInterval}
	retryIntervalDuration := &v1.Duration{Duration: durationRetry}

	opts := &helm.Options{
		SetValues: map[string]string{
			"users[0].name":                      name,
			"users[0].namespace":                 namespace,
			"users[0].cleanupPolicy":             cleanupPolicy,
			"users[0].host":                      host,
			"users[0].maxUserConnections":        strconv.Itoa(maxUserConnections),
			"users[0].passwordSecretKeyRef.key":  passwordSecretKeyRefKey,
			"users[0].passwordSecretKeyRef.name": passwordSecretKeyRefName,
			"users[0].requeueInterval":           requeueInterval,
			"users[0].retryInterval":             retryInterval,
			"users[0].mariaDBRef.name":           "cluster",
			"users[0].mariaDBRef.namespace":      "namespace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/user.yaml"})
	var user v1alpha1.User
	helm.UnmarshalK8SYaml(t, renderedData, &user)

	Expect(user.Name).To(Equal(fmt.Sprintf("%s-%s", clusterHelmReleaseName, name)))
	Expect(user.Namespace).To(Equal(namespace))
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

	name := "mariadb"
	namespace := "database"
	cleanupPolicy := "Delete"
	database := "mariadb"
	host := "%"
	username := "mariadb"
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
			"grants[0].name":                 name,
			"grants[0].namespace":            namespace,
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
			"grants[0].mariaDBRef.namespace": "namespace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/grant.yaml"})
	var grant v1alpha1.Grant
	helm.UnmarshalK8SYaml(t, renderedData, &grant)

	Expect(grant.Name).To(Equal(fmt.Sprintf("%s-%s", clusterHelmReleaseName, username)))
	Expect(grant.Namespace).To(Equal(namespace))
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

	delete(opts.SetValues, "grants[0].name")
	_, err := helm.RenderTemplateE(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/grant.yaml"})
	Expect(err).To(HaveOccurred())
}

func TestClusterHelmBackup(t *testing.T) {
	RegisterTestingT(t)

	name := "backup"
	namespace := "database"
	maxRetention := "720h"
	compression := "gzip"
	bucket := "backups"
	endpoint := "minio.minio.svc.cluster.local:9000"
	region := "us-east-1"
	prefix := "mariadb-cluster"
	accessKeyIdSecretKeyRefKey := "minio"
	accessKeyIdSecretKeyRefName := "access-key-id"
	secretAccessKeySecretKeyRefKey := "minio"
	secretAccessKeySecretKeyRefName := "secret-access-key"
	cron := "0 0 * * *"

	durationMaxRetention, _ := time.ParseDuration(maxRetention)
	durationMaxRetentionDuration := &v1.Duration{Duration: durationMaxRetention}

	opts := &helm.Options{
		SetValues: map[string]string{
			"backups[0].name":                                        name,
			"backups[0].namespace":                                   namespace,
			"backups[0].maxRetention":                                maxRetention,
			"backups[0].compression":                                 compression,
			"backups[0].storage.s3.bucket":                           bucket,
			"backups[0].storage.s3.prefix":                           prefix,
			"backups[0].storage.s3.endpoint":                         endpoint,
			"backups[0].storage.s3.region":                           region,
			"backups[0].storage.s3.accessKeyIdSecretKeyRef.key":      accessKeyIdSecretKeyRefKey,
			"backups[0].storage.s3.accessKeyIdSecretKeyRef.name":     accessKeyIdSecretKeyRefName,
			"backups[0].storage.s3.secretAccessKeySecretKeyRef.key":  secretAccessKeySecretKeyRefKey,
			"backups[0].storage.s3.secretAccessKeySecretKeyRef.name": secretAccessKeySecretKeyRefName,
			"backups[0].schedule.cron":                               cron,
			"backups[0].mariaDBRef.name":                             "cluster",
			"backups[0].mariaDBRef.namespace":                        "namespace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/backup.yaml"})
	var backup v1alpha1.Backup
	helm.UnmarshalK8SYaml(t, renderedData, &backup)

	Expect(backup.Name).To(Equal(fmt.Sprintf("%s-%s", clusterHelmReleaseName, name)))
	Expect(backup.Namespace).To(Equal(namespace))
	Expect(backup.Spec.MariaDBRef.Name).To(Equal(clusterHelmReleaseName))
	Expect(backup.Spec.MariaDBRef.Namespace).To(Equal(clusterHelmNamespace))
	Expect(&backup.Spec.MaxRetention).To(Equal(durationMaxRetentionDuration))
	Expect(backup.Spec.Compression).To(Equal(v1alpha1.CompressAlgorithm(compression)))
	Expect(backup.Spec.Storage.S3.Bucket).To(Equal(bucket))
	Expect(backup.Spec.Storage.S3.Prefix).To(Equal(prefix))
	Expect(backup.Spec.Storage.S3.Endpoint).To(Equal(endpoint))
	Expect(backup.Spec.Storage.S3.Region).To(Equal(region))
	Expect(backup.Spec.Storage.S3.AccessKeyIdSecretKeyRef.Key).To(Equal(accessKeyIdSecretKeyRefKey))
	Expect(backup.Spec.Storage.S3.AccessKeyIdSecretKeyRef.Name).To(Equal(accessKeyIdSecretKeyRefName))
	Expect(backup.Spec.Storage.S3.SecretAccessKeySecretKeyRef.Key).To(Equal(secretAccessKeySecretKeyRefKey))
	Expect(backup.Spec.Storage.S3.SecretAccessKeySecretKeyRef.Name).To(Equal(secretAccessKeySecretKeyRefName))
	Expect(backup.Spec.Schedule.Cron).To(Equal(cron))

	delete(opts.SetValues, "backups[0].name")
	delete(opts.SetValues, "backups[0].storage.s3.bucket")
	delete(opts.SetValues, "backups[0].storage.s3.prefix")
	delete(opts.SetValues, "backups[0].storage.s3.endpoint")
	delete(opts.SetValues, "backups[0].storage.s3.region")
	delete(opts.SetValues, "backups[0].storage.s3.accessKeyIdSecretKeyRef.key")
	delete(opts.SetValues, "backups[0].storage.s3.accessKeyIdSecretKeyRef.name")
	delete(opts.SetValues, "backups[0].storage.s3.secretAccessKeySecretKeyRef.key")
	delete(opts.SetValues, "backups[0].storage.s3.secretAccessKeySecretKeyRef.name")

	_, err := helm.RenderTemplateE(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/backup.yaml"})
	Expect(err).To(HaveOccurred())
}

func TestClusterHelmPhysicalBackup(t *testing.T) {
	RegisterTestingT(t)

	name := "physicalbackup"
	namespace := "database"
	maxRetention := "720h"
	compression := "gzip"
	bucket := "backups"
	endpoint := "minio.minio.svc.cluster.local:9000"
	region := "us-east-1"
	prefix := "mariadb-cluster"
	accessKeyIdSecretKeyRefKey := "minio"
	accessKeyIdSecretKeyRefName := "access-key-id"
	secretAccessKeySecretKeyRefKey := "minio"
	secretAccessKeySecretKeyRefName := "secret-access-key"
	cron := "0 0 * * *"

	durationMaxRetention, _ := time.ParseDuration(maxRetention)
	durationMaxRetentionDuration := &v1.Duration{Duration: durationMaxRetention}

	opts := &helm.Options{
		SetValues: map[string]string{
			"physicalBackups[0].name":                                        name,
			"physicalBackups[0].namespace":                                   namespace,
			"physicalBackups[0].maxRetention":                                maxRetention,
			"physicalBackups[0].compression":                                 compression,
			"physicalBackups[0].storage.s3.bucket":                           bucket,
			"physicalBackups[0].storage.s3.prefix":                           prefix,
			"physicalBackups[0].storage.s3.endpoint":                         endpoint,
			"physicalBackups[0].storage.s3.region":                           region,
			"physicalBackups[0].storage.s3.accessKeyIdSecretKeyRef.key":      accessKeyIdSecretKeyRefKey,
			"physicalBackups[0].storage.s3.accessKeyIdSecretKeyRef.name":     accessKeyIdSecretKeyRefName,
			"physicalBackups[0].storage.s3.secretAccessKeySecretKeyRef.key":  secretAccessKeySecretKeyRefKey,
			"physicalBackups[0].storage.s3.secretAccessKeySecretKeyRef.name": secretAccessKeySecretKeyRefName,
			"physicalBackups[0].schedule.cron":                               cron,
			"physicalBackups[0].mariaDBRef.name":                             "cluster",
			"physicalBackups[0].mariaDBRef.namespace":                        "namespace",
		},
	}

	renderedData := helm.RenderTemplate(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/physicalbackup.yaml"})
	var backup v1alpha1.Backup
	helm.UnmarshalK8SYaml(t, renderedData, &backup)

	Expect(backup.Name).To(Equal(fmt.Sprintf("%s-%s", clusterHelmReleaseName, name)))
	Expect(backup.Namespace).To(Equal(namespace))
	Expect(backup.Spec.MariaDBRef.Name).To(Equal(clusterHelmReleaseName))
	Expect(backup.Spec.MariaDBRef.Namespace).To(Equal(clusterHelmNamespace))
	Expect(&backup.Spec.MaxRetention).To(Equal(durationMaxRetentionDuration))
	Expect(backup.Spec.Compression).To(Equal(v1alpha1.CompressAlgorithm(compression)))
	Expect(backup.Spec.Storage.S3.Bucket).To(Equal(bucket))
	Expect(backup.Spec.Storage.S3.Prefix).To(Equal(prefix))
	Expect(backup.Spec.Storage.S3.Endpoint).To(Equal(endpoint))
	Expect(backup.Spec.Storage.S3.Region).To(Equal(region))
	Expect(backup.Spec.Storage.S3.AccessKeyIdSecretKeyRef.Key).To(Equal(accessKeyIdSecretKeyRefKey))
	Expect(backup.Spec.Storage.S3.AccessKeyIdSecretKeyRef.Name).To(Equal(accessKeyIdSecretKeyRefName))
	Expect(backup.Spec.Storage.S3.SecretAccessKeySecretKeyRef.Key).To(Equal(secretAccessKeySecretKeyRefKey))
	Expect(backup.Spec.Storage.S3.SecretAccessKeySecretKeyRef.Name).To(Equal(secretAccessKeySecretKeyRefName))
	Expect(backup.Spec.Schedule.Cron).To(Equal(cron))

	delete(opts.SetValues, "physicalBackups[0].name")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.bucket")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.prefix")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.endpoint")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.region")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.accessKeyIdSecretKeyRef.key")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.accessKeyIdSecretKeyRef.name")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.secretAccessKeySecretKeyRef.key")
	delete(opts.SetValues, "physicalBackups[0].storage.s3.secretAccessKeySecretKeyRef.name")

	_, err := helm.RenderTemplateE(t, opts, clusterHelmChartPath, clusterHelmReleaseName, []string{"templates/physicalbackup.yaml"})
	Expect(err).To(HaveOccurred())
}
