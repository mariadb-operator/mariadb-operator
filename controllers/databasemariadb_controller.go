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

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/conditions"
	"github.com/mmontes11/mariadb-operator/pkg/controller/template"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	"github.com/mmontes11/mariadb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DatabaseMariaDBReconciler reconciles a DatabaseMariaDB object
type DatabaseMariaDBReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
}

//+kubebuilder:rbac:groups=database.mmontes.io,resources=databasemariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.mmontes.io,resources=databasemariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.mmontes.io,resources=databasemariadbs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DatabaseMariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var database databasev1alpha1.DatabaseMariaDB
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	wr := newWrappedDatabaseReconciler(r.Client, r.RefResolver, &database)
	wf := newWrappedDatabaseFinalizer(r.Client, &database)
	tf := template.NewTemplateFinalizer(r.RefResolver, wf)
	tr := template.NewTemplateReconciler(r.RefResolver, r.ConditionReady, wr, tf)

	result, err := tr.Reconcile(ctx, &database)
	if err != nil {
		return result, fmt.Errorf("error reconciling in TemplateReconciler: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseMariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.DatabaseMariaDB{}).
		Complete(r)
}

type wrappedDatabaseReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	database    *databasev1alpha1.DatabaseMariaDB
}

func newWrappedDatabaseReconciler(client client.Client, refResolver *refresolver.RefResolver,
	database *databasev1alpha1.DatabaseMariaDB) template.WrappedReconciler {
	return &wrappedDatabaseReconciler{
		Client:      client,
		refResolver: refResolver,
		database:    database,
	}
}

func (wr *wrappedDatabaseReconciler) Reconcile(ctx context.Context, mdbClient *mariadbclient.Client) error {
	opts := mariadbclient.DatabaseOpts{
		CharacterSet: wr.database.Spec.CharacterSet,
		Collate:      wr.database.Spec.Collate,
	}
	if err := mdbClient.CreateDatabase(ctx, wr.database.Name, opts); err != nil {
		return fmt.Errorf("error creating database in MariaDB: %v", err)
	}
	return nil
}

func (wr *wrappedDatabaseReconciler) PatchStatus(ctx context.Context, patcher conditions.Patcher) error {
	patch := client.MergeFrom(wr.database.DeepCopy())
	patcher(&wr.database.Status)

	if err := wr.Client.Status().Patch(ctx, wr.database, patch); err != nil {
		return fmt.Errorf("error patching DatabaseMariaDB status: %v", err)
	}
	return nil
}
