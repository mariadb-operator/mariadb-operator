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
	defaultTagName        = "webhook"
	jsonTagName           = "json"
	inmutableTagValue     = "inmutable"
	inmutableInitTagValue = "inmutableinit"
)

type Option func(w *InmutableWebhook)

func WithTagName(tagName string) Option {
	return func(w *InmutableWebhook) {
		w.tagName = tagName
	}
}

type InmutableWebhook struct {
	tagName string
}

func NewInmutableWebhook(opts ...Option) *InmutableWebhook {
	webhook := &InmutableWebhook{
		tagName: defaultTagName,
	}
	for _, setOpt := range opts {
		setOpt(webhook)
	}
	return webhook
}

func (w *InmutableWebhook) ValidateUpdate(new, old client.Object) error {
	specVal := specValue(new)
	oldSpecVal := specValue(old)

	errBundle := w.validateInmutable(specVal, oldSpecVal, "spec")

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

func (w *InmutableWebhook) validateInmutable(val, oldVal reflect.Value, pathElements ...string) field.ErrorList {
	var errBundle field.ErrorList

	for i := 0; i < val.NumField(); i++ {
		fieldStruct := val.Type().Field(i)
		fieldVal := val.Field(i)
		oldFieldVal := oldVal.Field(i)

		_, modifier := jsonParts(fieldStruct)
		if modifier == "inline" {
			inlineErrors := w.validateInmutable(fieldVal, oldFieldVal, pathElements...)
			if inlineErrors != nil {
				errBundle = append(errBundle, inlineErrors...)
			}
		}

		fieldIface := fieldVal.Interface()
		oldFieldIface := oldFieldVal.Interface()

		tag := fieldStruct.Tag.Get(w.tagName)
		switch tag {
		case inmutableTagValue:
			if !reflect.DeepEqual(fieldIface, oldFieldIface) {
				errBundle = append(errBundle, inmutableFieldError(fieldStruct, fieldIface, pathElements...))
			}
		case inmutableInitTagValue:
			if !oldFieldVal.IsNil() && !reflect.DeepEqual(fieldIface, oldFieldIface) {
				errBundle = append(errBundle, inmutableFieldError(fieldStruct, fieldIface, pathElements...))
			}
		}
	}
	return errBundle
}

func inmutableFieldError(structField reflect.StructField, value interface{}, pathElements ...string) *field.Error {
	var path *field.Path
	jsonField, _ := jsonParts(structField)
	if jsonField != "" && len(pathElements) > 0 {
		var moreNames []string
		if len(pathElements) > 1 {
			moreNames = pathElements[1:]
		}
		path = field.NewPath(pathElements[0], moreNames...).Child(jsonField)
	} else {
		path = field.NewPath(structField.Name)
	}
	return field.Invalid(
		path,
		value,
		fmt.Sprintf("'%s' field is inmutable", path.String()),
	)
}

func specValue(obj client.Object) reflect.Value {
	ptr := reflect.ValueOf(obj)
	val := reflect.Indirect(ptr)
	return val.FieldByName("Spec")
}

func jsonParts(structField reflect.StructField) (field string, modifier string) {
	json := structField.Tag.Get(jsonTagName)
	if json == "" {
		return "", ""
	}
	parts := strings.Split(json, ",")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
