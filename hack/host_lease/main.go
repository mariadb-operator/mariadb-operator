// host_lease acquires (or renews) and releases the Lease that grants a single checkout the use of the host mode cluster.
// Usage: CLUSTER=<cluster> go run ./hack/host_lease <acquire|release>
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coordinationclientv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

const (
	leaseName      = "mariadb-operator-dev"
	leaseNamespace = "default"
	leaseDuration  = 4 * time.Hour
)

func main() {
	if len(os.Args) != 2 {
		exit("usage: host_lease <acquire|release>")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cluster := os.Getenv("CLUSTER")

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: "kind-" + cluster},
	).ClientConfig()
	if err != nil {
		exit("error getting kubeconfig for cluster '%s': %v", cluster, err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		exit("error creating client: %v", err)
	}
	leases := clientset.CoordinationV1().Leases(leaseNamespace)

	hostname, err := os.Hostname()
	if err != nil {
		exit("error getting hostname: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		exit("error getting working directory: %v", err)
	}
	holder := hostname + ":" + wd

	switch os.Args[1] {
	case "acquire":
		err = acquire(ctx, leases, cluster, holder)
	case "release":
		err = release(ctx, leases, cluster, holder)
	default:
		err = fmt.Errorf("unknown action '%s'", os.Args[1])
	}
	if err != nil {
		exit("%v", err)
	}
}

func acquire(ctx context.Context, leases coordinationclientv1.LeaseInterface, cluster, holder string) error {
	now := metav1.NowMicro()
	lease, err := leases.Get(ctx, leaseName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: leaseName, Namespace: leaseNamespace},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       ptr.To(holder),
				LeaseDurationSeconds: ptr.To(int32(leaseDuration.Seconds())),
				AcquireTime:          &now,
				RenewTime:            &now,
			},
		}
		if _, err := leases.Create(ctx, lease, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("error acquiring host mode lease of cluster '%s': %v", cluster, err)
		}
		fmt.Printf("Acquired host mode lease of cluster '%s' for %s.\n", cluster, leaseDuration)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error getting host mode lease of cluster '%s'. Is the cluster running ('make cluster')? %v", cluster, err)
	}

	currentHolder := ptr.Deref(lease.Spec.HolderIdentity, "")
	expiresAt := lease.Spec.RenewTime.Add(time.Duration(ptr.Deref(lease.Spec.LeaseDurationSeconds, 0)) * time.Second)
	if currentHolder != holder && time.Now().Before(expiresAt) {
		return fmt.Errorf("GUARD: host mode cluster '%s' is leased by '%s' until %s. "+
			"Consider using the in-cluster mode instead: 'make cluster-init'", cluster, currentHolder, expiresAt.Format(time.RFC3339))
	}
	if currentHolder != holder {
		lease.Spec.AcquireTime = &now
	}
	lease.Spec.HolderIdentity = ptr.To(holder)
	lease.Spec.LeaseDurationSeconds = ptr.To(int32(leaseDuration.Seconds()))
	lease.Spec.RenewTime = &now
	// Update relies on the resourceVersion, so it fails if another checkout updated the lease in the meantime.
	if _, err := leases.Update(ctx, lease, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("error renewing host mode lease of cluster '%s': %v", cluster, err)
	}
	return nil
}

func release(ctx context.Context, leases coordinationclientv1.LeaseInterface, cluster, holder string) error {
	lease, err := leases.Get(ctx, leaseName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || (err == nil && ptr.Deref(lease.Spec.HolderIdentity, "") != holder) {
		fmt.Printf("Host mode lease of cluster '%s' is not held by this checkout.\n", cluster)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error getting host mode lease of cluster '%s': %v", cluster, err)
	}
	if err := leases.Delete(ctx, leaseName, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{ResourceVersion: &lease.ResourceVersion},
	}); err != nil {
		return fmt.Errorf("error releasing host mode lease of cluster '%s': %v", cluster, err)
	}
	fmt.Printf("Released host mode lease of cluster '%s'.\n", cluster)
	return nil
}

func exit(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
