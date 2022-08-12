package webhook

import (
	"fmt"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultTagName  = "webhook"
	defaultTagValue = "inmutable"
)

type Option func(w *InmutableWebhook)

func WithTagName(tagName string) Option {
	return func(w *InmutableWebhook) {
		w.tagName = tagName
	}
}

func WithTagValue(tagValue string) Option {
	return func(w *InmutableWebhook) {
		w.tagValue = tagValue
	}
}

type InmutableWebhook struct {
	tagName  string
	tagValue string
}

func NewInmutableWebhook(opts ...Option) *InmutableWebhook {
	webhook := &InmutableWebhook{
		tagName:  defaultTagName,
		tagValue: defaultTagValue,
	}
	for _, setOpt := range opts {
		setOpt(webhook)
	}
	return webhook
}

func (w *InmutableWebhook) ValidateUpdate(new, old client.Object) error {
	var errBundle field.ErrorList
	newSpec := getSpecField(new)
	oldSpec := getSpecField(old)
	t := newSpec.Type()

	for i := 0; i < newSpec.NumField(); i++ {
		newField := t.Field(i)
		tag := newField.Tag.Get(w.tagName)
		if tag != w.tagValue {
			continue
		}

		newVal := newSpec.Field(i).Interface()
		oldVal := oldSpec.Field(i).Interface()
		if !reflect.DeepEqual(newVal, oldVal) {
			errBundle = append(errBundle, getInmutableFieldError(newField, newVal))
		}
	}

	if len(errBundle) == 0 {
		return nil
	}
	gvk := new.GetObjectKind().GroupVersionKind()
	return apierrors.NewInvalid(
		schema.GroupKind{
			Group: gvk.Group,
			Kind:  gvk.Kind,
		},
		new.GetName(),
		errBundle,
	)
}

func getSpecField(obj client.Object) reflect.Value {
	ptr := reflect.ValueOf(obj)
	val := reflect.Indirect(ptr)
	return val.FieldByName("Spec")
}

func getInmutableFieldError(structField reflect.StructField, value interface{}) *field.Error {
	var path *field.Path
	if json := structField.Tag.Get("json"); json != "" {
		parts := strings.Split(json, ",")
		path = field.NewPath("spec").Child(parts[0])
	} else {
		path = field.NewPath(structField.Name)
	}
	return field.Invalid(
		path,
		value,
		fmt.Sprintf("'%s' field is inmutable", path.String()),
	)
}
