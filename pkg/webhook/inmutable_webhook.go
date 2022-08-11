package webhook

import (
	"fmt"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultTagName  = "webhook"
	defaultTagValue = "inmutable"
)

type options struct {
	tagName  string
	tagValue string
}

type Option func(o *options)

func WithTagName(tagName string) Option {
	return func(o *options) {
		o.tagName = tagName
	}
}

func WithTagValue(tagValue string) Option {
	return func(o *options) {
		o.tagValue = tagValue
	}
}

type InmutableWebhook[T client.Object] struct {
	options
}

func NewInmutableWebhook[T client.Object](opts ...Option) *InmutableWebhook[T] {
	options := options{
		tagName:  defaultTagName,
		tagValue: defaultTagValue,
	}
	for _, setOpt := range opts {
		setOpt(&options)
	}

	return &InmutableWebhook[T]{
		options: options,
	}
}

func (w *InmutableWebhook[T]) ValidateUpdate(new, old T) error {
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

func getSpecField[T runtime.Object](restore T) reflect.Value {
	ptr := reflect.ValueOf(restore)
	val := reflect.Indirect(ptr)
	return val.FieldByName("Spec")
}

func getInmutableFieldError(structField reflect.StructField, value interface{}) *field.Error {
	var path *field.Path
	json := structField.Tag.Get("json")
	if json != "" {
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
