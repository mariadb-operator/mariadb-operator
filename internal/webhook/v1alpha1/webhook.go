package v1alpha1

import (
	"fmt"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/webhook"
	cron "github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	inmutableWebhook = webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
	)
	cronParser = cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)
)

type tlsValidationItem struct {
	tlsValue            interface{}
	caSecretRef         *v1alpha1.LocalObjectReference
	caFieldPath         string
	certSecretRef       *v1alpha1.LocalObjectReference
	certFieldPath       string
	certIssuerRef       *cmmeta.ObjectReference
	certIssuerFieldPath string
}

func validateTLSCert(item *tlsValidationItem) error {
	if item.certSecretRef != nil && item.certIssuerRef != nil {
		return field.Invalid(
			field.NewPath("spec").Child("tls"),
			item.tlsValue,
			fmt.Sprintf(
				"'%s' and '%s' are mutually exclusive. Only one of them must be set at a time.",
				item.certFieldPath,
				item.certIssuerFieldPath,
			),
		)
	}
	if item.caSecretRef == nil && item.certSecretRef != nil {
		return field.Invalid(
			field.NewPath("spec").Child("tls"),
			item.tlsValue,
			fmt.Sprintf("'%s' must be set when '%s' is set", item.caFieldPath, item.certFieldPath),
		)
	}
	return nil
}
