package strategy

import (
	"fmt"
)

type ParamType string

const (
	ParamTypeInt     ParamType = "int"
	ParamTypeFloat   ParamType = "float"
	ParamTypeString  ParamType = "string"
	ParamTypeBool    ParamType = "bool"
	ParamTypeStringSlice ParamType = "string_slice"
)

type ParamDefinition struct {
	Name         string      `json:"name"`
	Type         ParamType   `json:"type"`
	DefaultValue interface{} `json:"default_value"`
	MinValue     interface{} `json:"min_value,omitempty"`
	MaxValue     interface{} `json:"max_value,omitempty"`
	Required     bool        `json:"required"`
	Description  string      `json:"description"`
}

type ParamSchema struct {
	StrategyName string           `json:"strategy_name"`
	Params       []ParamDefinition `json:"params"`
}

type ParamValidator struct {
	schema ParamSchema
}

func NewParamValidator(schema ParamSchema) *ParamValidator {
	return &ParamValidator{
		schema: schema,
	}
}

func (pv *ParamValidator) Validate(params map[string]interface{}) error {
	for _, def := range pv.schema.Params {
		value, exists := params[def.Name]
		
		if def.Required && !exists {
			return fmt.Errorf("required parameter '%s' not found", def.Name)
		}
		
		if !exists {
			continue
		}
		
		if err := pv.validateParamType(def, value); err != nil {
			return err
		}
		
		if err := pv.validateParamRange(def, value); err != nil {
			return err
		}
	}
	
	return nil
}

func (pv *ParamValidator) validateParamType(def ParamDefinition, value interface{}) error {
	switch def.Type {
	case ParamTypeInt:
		if !isIntType(value) {
			return fmt.Errorf("parameter '%s' must be int, got %T", def.Name, value)
		}
	case ParamTypeFloat:
		if !isFloatType(value) {
			return fmt.Errorf("parameter '%s' must be float, got %T", def.Name, value)
		}
	case ParamTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter '%s' must be string, got %T", def.Name, value)
		}
	case ParamTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter '%s' must be bool, got %T", def.Name, value)
		}
	case ParamTypeStringSlice:
		if !isStringSliceType(value) {
			return fmt.Errorf("parameter '%s' must be string slice, got %T", def.Name, value)
		}
	}
	return nil
}

func (pv *ParamValidator) validateParamRange(def ParamDefinition, value interface{}) error {
	switch def.Type {
	case ParamTypeInt:
		intVal := toInt(value)
		if def.MinValue != nil {
			minVal := toInt(def.MinValue)
			if intVal < minVal {
				return fmt.Errorf("parameter '%s' must be >= %d, got %d", def.Name, minVal, intVal)
			}
		}
		if def.MaxValue != nil {
			maxVal := toInt(def.MaxValue)
			if intVal > maxVal {
				return fmt.Errorf("parameter '%s' must be <= %d, got %d", def.Name, maxVal, intVal)
			}
		}
	case ParamTypeFloat:
		floatVal := toFloat(value)
		if def.MinValue != nil {
			minVal := toFloat(def.MinValue)
			if floatVal < minVal {
				return fmt.Errorf("parameter '%s' must be >= %.4f, got %.4f", def.Name, minVal, floatVal)
			}
		}
		if def.MaxValue != nil {
			maxVal := toFloat(def.MaxValue)
			if floatVal > maxVal {
				return fmt.Errorf("parameter '%s' must be <= %.4f, got %.4f", def.Name, maxVal, floatVal)
			}
		}
	}
	return nil
}

func (pv *ParamValidator) ApplyDefaults(params map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range params {
		result[k] = v
	}
	
	for _, def := range pv.schema.Params {
		if _, exists := result[def.Name]; !exists && def.DefaultValue != nil {
			result[def.Name] = def.DefaultValue
		}
	}
	
	return result
}

func (pv *ParamValidator) GetSchema() ParamSchema {
	return pv.schema
}

func isIntType(v interface{}) bool {
	switch v.(type) {
	case int, int64, int32, float64, float32:
		return true
	}
	return false
}

func isFloatType(v interface{}) bool {
	switch v.(type) {
	case float64, float32, int, int64, int32:
		return true
	}
	return false
}

func isStringSliceType(v interface{}) bool {
	switch v.(type) {
	case []string:
		return true
	case []interface{}:
		slice, ok := v.([]interface{})
		if !ok {
			return false
		}
		for _, item := range slice {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	}
	return false
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return 0
	}
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0.0
	}
}

type StrategyWithSchema interface {
	Strategy
	GetParamSchema() ParamSchema
}

func GetParamDefinitions(s Strategy) []ParamDefinition {
	if sws, ok := s.(StrategyWithSchema); ok {
		return sws.GetParamSchema().Params
	}
	return nil
}
