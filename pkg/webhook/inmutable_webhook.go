package webhook

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"

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
	specStructField, err := specType(new)
	if err != nil {
		return err
	}
	specVal := specValue(new)
	oldSpecVal := specValue(old)

	errBundle := w.validateInmutable(specStructField, specVal, oldSpecVal, "spec")

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

func (w *InmutableWebhook) validateInmutable(structField reflect.StructField, val, oldVal reflect.Value,
	pathElements ...string) field.ErrorList {
	var errBundle field.ErrorList

	if !isValidStruct(val) || !isValidStruct(oldVal) {
		if err := w.validateInmutableValue(structField, val, oldVal, pathElements...); err != nil {
			return []*field.Error{err}
		}
		return nil
	}
	structVal := reflect.Indirect(val)
	structOldVal := reflect.Indirect(oldVal)

	for i := 0; i < structVal.NumField() && i < structOldVal.NumField(); i++ {
		fieldStruct := structVal.Type().Field(i)
		fieldVal := structVal.Field(i)
		fieldOldVal := structOldVal.Field(i)

		if isValidStruct(fieldVal) {
			nestedErrors := w.validateInmutable(
				fieldStruct,
				fieldVal,
				fieldOldVal,
				appendField(fieldStruct, pathElements)...,
			)
			if nestedErrors != nil {
				errBundle = append(errBundle, nestedErrors...)
			}
		}
		if fieldVal.Kind() == reflect.Slice {
			for j := 0; j < fieldVal.Len() && j < fieldOldVal.Len(); j++ {
				sliceElement := fieldVal.Index(j)
				sliceOldElement := fieldOldVal.Index(j)

				if isValidStruct(sliceElement) {
					nestedErrors := w.validateInmutable(
						fieldStruct,
						sliceElement,
						sliceOldElement,
						appendField(fieldStruct, pathElements)...,
					)
					if nestedErrors != nil {
						errBundle = append(errBundle, nestedErrors...)
					}
				}
			}
		}

		if err := w.validateInmutableValue(fieldStruct, fieldVal, fieldOldVal, pathElements...); err != nil {
			errBundle = append(errBundle, err)
		}
	}
	return errBundle
}

func (w *InmutableWebhook) validateInmutableValue(structField reflect.StructField, val, oldVal reflect.Value,
	pathElements ...string) *field.Error {
	// calling .Interface() on an unexported field results in a panic
	if isUnexported(structField.Name) {
		return nil
	}
	tag := structField.Tag.Get(w.tagName)
	fieldIface := val.Interface()
	oldFieldIface := oldVal.Interface()

	switch tag {
	case inmutableTagValue:
		if !reflect.DeepEqual(fieldIface, oldFieldIface) {
			return inmutableFieldError(structField, fieldIface, pathElements...)
		}
	case inmutableInitTagValue:
		if !isNilOrZero(oldVal) && !reflect.DeepEqual(fieldIface, oldFieldIface) {
			return inmutableFieldError(structField, fieldIface, pathElements...)
		}
	}
	return nil
}

func specType(obj client.Object) (reflect.StructField, error) {
	ptr := reflect.ValueOf(obj)
	val := reflect.Indirect(ptr)
	specField, ok := val.Type().FieldByName("Spec")
	if !ok {
		return reflect.StructField{}, errors.New("'spec' field not found")
	}
	return specField, nil
}

func specValue(obj client.Object) reflect.Value {
	ptr := reflect.ValueOf(obj)
	val := reflect.Indirect(ptr)
	return val.FieldByName("Spec")
}

func isNilOrZero(val reflect.Value) bool {
	if val.Kind() == reflect.Ptr {
		return val.IsNil()
	}
	return val.IsZero()
}

func isValidStruct(val reflect.Value) bool {
	if isNilOrZero(val) {
		return false
	}
	return reflect.Indirect(val).Kind() == reflect.Struct
}

func inmutableFieldError(structField reflect.StructField, value interface{}, pathElements ...string) *field.Error {
	var path *field.Path
	jsonField := jsonField(structField)
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

func jsonField(structField reflect.StructField) string {
	json := structField.Tag.Get(jsonTagName)
	if json == "" {
		return ""
	}
	parts := strings.Split(json, ",")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func appendField(structField reflect.StructField, pathElements []string) []string {
	field := jsonField(structField)
	if field == "" {
		return pathElements
	}
	return append(pathElements, field)
}

func isUnexported(s string) bool {
	if len(s) == 0 {
		return false
	}
	return unicode.IsLower(rune(s[0]))
}
