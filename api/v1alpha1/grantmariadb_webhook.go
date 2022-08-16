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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *GrantMariaDB) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//nolint
//+kubebuilder:webhook:path=/validate-database-mmontes-io-v1alpha1-grantmariadb,mutating=false,failurePolicy=fail,sideEffects=None,groups=database.mmontes.io,resources=grantmariadbs,verbs=create;update,versions=v1alpha1,name=vgrantmariadb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GrantMariaDB{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *GrantMariaDB) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *GrantMariaDB) ValidateUpdate(old runtime.Object) error {
	return inmutableWebhook.ValidateUpdate(r, old.(*GrantMariaDB))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *GrantMariaDB) ValidateDelete() error {
	return nil
}
