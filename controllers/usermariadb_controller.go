/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"sync"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/controllers/template"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// UserMariaDBReconciler reconciles a UserMariaDB object
type UserMariaDBReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready

	wfPool sync.Pool
	wrPool sync.Pool
	tfPool sync.Pool
	trPool sync.Pool
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=usermariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=usermariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=usermariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var user databasev1alpha1.UserMariaDB
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, ctrlClient.IgnoreNotFound(err)
	}

	tr, backToPool := r.getTemplateReconciler(&user)
	defer backToPool()

	result, err := tr.Reconcile(ctx, &user)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

func (r *UserMariaDBReconciler) getTemplateReconciler(user *databasev1alpha1.UserMariaDB) (template.Reconciler, func()) {
	wf := r.wfPool.Get().(*wrappedUserFinalizer)
	wf.user = user

	wr := r.wrPool.Get().(*wrappedUserReconciler)
	wr.user = user

	tf := r.tfPool.Get().(*template.TemplateFinalizer)
	tf.WrappedFinalizer = wf

	tr := r.trPool.Get().(*template.TemplateReconciler)
	tr.WrappedReconciler = wr
	tr.Finalizer = tf

	return tr, func() {
		r.wfPool.Put(wf)
		r.wrPool.Put(wr)
		r.tfPool.Put(tf)
		r.trPool.Put(tr)
	}
}

func (r *UserMariaDBReconciler) initPool() {
	r.wfPool = sync.Pool{
		New: func() interface{} {
			return &wrappedUserFinalizer{
				Client: r.Client,
			}
		},
	}
	r.wrPool = sync.Pool{
		New: func() interface{} {
			return &wrappedUserReconciler{
				Client:      r.Client,
				refResolver: r.RefResolver,
			}
		},
	}
	r.tfPool = sync.Pool{
		New: func() interface{} {
			return &template.TemplateFinalizer{
				RefResolver: r.RefResolver,
			}
		},
	}
	r.trPool = sync.Pool{
		New: func() interface{} {
			return &template.TemplateReconciler{
				RefResolver:    r.RefResolver,
				ConditionReady: r.ConditionReady,
			}
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.initPool()

	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.UserMariaDB{}).
		Complete(r)
}

type wrappedUserReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	user        *databasev1alpha1.UserMariaDB
}

func (wr *wrappedUserReconciler) Reconcile(ctx context.Context, mdbClient *mariadbclient.Client) error {
	password, err := wr.refResolver.ReadSecretKeyRef(ctx, wr.user.Spec.PasswordSecretKeyRef, wr.user.Namespace)
	if err != nil {
		return fmt.Errorf("error reading user password secret: %v", err)
	}
	opts := mariadbclient.CreateUserOpts{
		IdentifiedBy:       password,
		MaxUserConnections: wr.user.Spec.MaxUserConnections,
	}
	if err := mdbClient.CreateUser(ctx, wr.user.Name, opts); err != nil {
		return fmt.Errorf("error creating user in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedUserReconciler) PatchStatus(ctx context.Context, patcher conditions.Patcher) error {
	patch := client.MergeFrom(wr.user.DeepCopy())
	patcher(&wr.user.Status)

	if err := wr.Client.Status().Patch(ctx, wr.user, patch); err != nil {
		return fmt.Errorf("error patching UserMariaDB status: %v", err)
	}
	return nil
}
