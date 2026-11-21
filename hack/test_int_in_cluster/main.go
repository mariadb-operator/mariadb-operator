// test_int_in_cluster runs the integration tests image as a Job in the in-cluster mode slot cluster, streams its logs
// and exits with its result. The Job runs as the operator ServiceAccount bound to cluster-admin, so it can both manage
// the cluster and authenticate against the data-plane agent. Azurite is seeded first by a Job running the same image, as the
// containers can only be reached through the cluster DNS.
// Usage: CLUSTER=<cluster> TEST_IMG=<image> TEST_ENV='<KEY=VALUE ...>' go run ./hack/test_int_in_cluster [test binary flags]
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"

	"github.com/mariadb-operator/mariadb-operator/v26/pkg/docker"
)

const (
	name       = "mariadb-operator-test-int"
	seedName   = "mariadb-operator-test-int-azurite"
	seedBinary = "/test/azurite-seed"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: "kind-" + os.Getenv("CLUSTER")},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("error getting kubeconfig: %v", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("error creating client: %v", err)
	}
	vars, err := testEnv(ctx)
	if err != nil {
		return err
	}
	namespace := vars["MARIADB_OPERATOR_NAMESPACE"]
	serviceAccount := vars["MARIADB_OPERATOR_NAME"]
	env := envVars(vars)

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: serviceAccount, Namespace: namespace}},
	}
	if _, err := client.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil &&
		!apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating ClusterRoleBinding: %v", err)
	}
	// The CRB is only needed while the Job runs, so remove it when done. The Jobs are kept, to inspect their logs later.
	defer func() {
		if err := client.RbacV1().ClusterRoleBindings().Delete(ctx, name, metav1.DeleteOptions{}); err != nil &&
			!apierrors.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "warning: error deleting ClusterRoleBinding %s: %v\n", name, err)
		}
	}()

	if seeded, err := azuriteSeeded(ctx, client, namespace); err != nil {
		return err
	} else if !seeded {
		if err := runJob(ctx, client, seedJob(namespace, serviceAccount, env)); err != nil {
			return err
		}
	}
	return runJob(ctx, client, job(namespace, serviceAccount, env))
}

// runJob runs a Job, replacing any previous one, streams its logs and returns an error if it does not succeed.
func runJob(ctx context.Context, client kubernetes.Interface, j *batchv1.Job) error {
	namespace, jobName := j.Namespace, j.Name
	jobs := client.BatchV1().Jobs(namespace)
	err := jobs.Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationForeground)})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error deleting previous Job: %v", err)
	}
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := jobs.Get(ctx, jobName, metav1.GetOptions{})
		return apierrors.IsNotFound(err), nil
	}); err != nil {
		return fmt.Errorf("error waiting for previous Job to be deleted: %v", err)
	}

	if _, err := jobs.Create(ctx, j, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("error creating Job: %v", err)
	}
	fmt.Printf("Job '%s/%s' created.\n", namespace, jobName)

	var pod string
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
		if err != nil || len(pods.Items) == 0 || pods.Items[0].Status.Phase == corev1.PodPending {
			return false, nil
		}
		pod = pods.Items[0].Name
		return true, nil
	}); err != nil {
		return fmt.Errorf("error waiting for Job Pod to start: %v", err)
	}

	logs, err := client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
	if err != nil {
		return fmt.Errorf("error streaming logs: %v", err)
	}
	defer logs.Close()
	_, _ = io.Copy(os.Stdout, logs)

	var result *batchv1.Job
	if err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		result, err = jobs.Get(ctx, jobName, metav1.GetOptions{})
		return err == nil && (result.Status.Succeeded > 0 || result.Status.Failed > 0), nil
	}); err != nil {
		return fmt.Errorf("error waiting for Job to finish: %v", err)
	}
	if result.Status.Succeeded == 0 {
		return fmt.Errorf("job '%s/%s' failed", namespace, jobName)
	}
	fmt.Printf("Job '%s/%s' succeeded.\n", namespace, jobName)
	return nil
}

// job runs the tests, passing the arguments received by this program to the compiled test binary as they are.
func job(namespace, serviceAccount string, env []corev1.EnvVar) *batchv1.Job {
	return newJob(name, namespace, serviceAccount, nil, os.Args[1:], env)
}

// seedJob creates the Azurite containers used by the tests.
func seedJob(namespace, serviceAccount string, env []corev1.EnvVar) *batchv1.Job {
	return newJob(seedName, namespace, serviceAccount, []string{seedBinary}, nil, env)
}

// azuriteSeeded returns whether a previous seed Job succeeded after the Azurite volume was created, so the containers,
// persisted in the volume, already exist.
func azuriteSeeded(ctx context.Context, client kubernetes.Interface, namespace string) (bool, error) {
	job, err := client.BatchV1().Jobs(namespace).Get(ctx, seedName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || (err == nil && job.Status.CompletionTime == nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error getting Azurite seed Job: %v", err)
	}
	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, "azurite-data", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("error getting Azurite PVC: %v", err)
	}
	return job.Status.CompletionTime.After(pvc.CreationTimestamp.Time), nil
}

// testEnv parses TEST_ENV, the same 'KEY=VALUE ...' variables passed to the tests in host mode (ENV in make/dev.mk),
// overriding the ones that differ inside the cluster.
func testEnv(ctx context.Context) (map[string]string, error) {
	vars := make(map[string]string)
	for _, kv := range strings.Fields(os.Getenv("TEST_ENV")) {
		key, value, _ := strings.Cut(kv, "=")
		vars[key] = strings.Trim(value, `"`)
	}
	vars["MARIADB_OPERATOR_SA_PATH"] = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	cidrPrefix, err := docker.GetKindCidrPrefix(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting KIND CIDR prefix: %v", err)
	}
	vars["TEST_CIDR_PREFIX"] = cidrPrefix
	return vars, nil
}

func envVars(vars map[string]string) []corev1.EnvVar {
	var env []corev1.EnvVar
	for key, value := range vars {
		env = append(env, corev1.EnvVar{Name: key, Value: value})
	}
	sort.Slice(env, func(i, j int) bool { return env[i].Name < env[j].Name })
	return env
}

func newJob(jobName, namespace, serviceAccount string, command, args []string, vars []corev1.EnvVar) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: namespace},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(0)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            "test-int",
						Image:           os.Getenv("TEST_IMG"),
						ImagePullPolicy: corev1.PullNever,
						Command:         command,
						Args:            args,
						Env:             vars,
					}},
				},
			},
		},
	}
}
