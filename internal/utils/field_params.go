package utils

import (
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/pkg/errors"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ParamValueSetting struct {
	ketchv1.Parameter
	Value interface{}
}

var (
	ErrInvalidParameterType = func(str string) error { return errors.Errorf("invalid parameter type: %s", str) }
)

func NewParameterValuesObject() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: make(map[string]interface{})}
}

func resolveKubeParameters(params []ketchv1.Parameter, settings map[string]interface{}) (map[string]ParamValueSetting, error) {
	values := make(map[string]ParamValueSetting)
	supported := map[string]*ketchv1.Parameter{}
	for _, p := range params {
		supported[p.Name] = p.DeepCopy()
	}
	for name, v := range settings {
		if supported[name] == nil {
			return nil, errors.Errorf("unsupported parameter %s", name)
		}
		values[name] = ParamValueSetting{
			Parameter: *supported[name],
			Value:     v,
		}
	}
	return values, nil
}

func setParameterValuesToKubeObj(obj *unstructured.Unstructured, parameterValueSettings map[string]ParamValueSetting) error {
	paved := fieldpath.Pave(obj.Object)
	for paramName, v := range parameterValueSettings {
		for _, f := range v.FieldPaths {
			switch v.Type {
			case "string":
				vString, ok := v.Value.(string)
				if !ok {
					return ErrInvalidParameterType(v.Type)
				}
				if err := paved.SetString(f, vString); err != nil {
					return errors.Wrapf(err, "cannot set parameter %q to field %q", paramName, f)
				}
			case "number", "int", "float":
				switch v.Value.(type) {
				case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
					if err := paved.SetValue(f, v.Value); err != nil {
						return errors.Wrapf(err, "cannot set parameter %q to field %q", paramName, f)
					}
				default:
					return ErrInvalidParameterType(v.Type)
				}
			case "boolean", "bool":
				vBoolean, ok := v.Value.(bool)
				if !ok {
					ErrInvalidParameterType(v.Type)
				}
				if err := paved.SetValue(f, vBoolean); err != nil {
					return errors.Wrapf(err, "cannot set parameter %q to field %q", paramName, f)
				}
			default:
				return ErrInvalidParameterType(v.Type)
			}
		}
	}
	return nil
}
