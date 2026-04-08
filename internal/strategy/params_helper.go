package strategy

import (
	"fmt"
)

func getFloat64(params map[string]interface{}, key string, defaultValue float64) float64 {
	if v, ok := params[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
		if f, ok := v.(int); ok {
			return float64(f)
		}
		if f, ok := v.(float32); ok {
			return float64(f)
		}
	}
	return defaultValue
}

func getInt(params map[string]interface{}, key string, defaultValue int) int {
	if v, ok := params[key]; ok {
		if i, ok := v.(int); ok {
			return i
		}
		if i, ok := v.(int64); ok {
			return int(i)
		}
		if i, ok := v.(float64); ok {
			return int(i)
		}
	}
	return defaultValue
}

func getString(params map[string]interface{}, key string, defaultValue string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getStringSlice(params map[string]interface{}, key string, defaultValue []string) []string {
	if v, ok := params[key]; ok {
		if s, ok := v.([]string); ok {
			return s
		}
		if s, ok := v.([]interface{}); ok {
			result := make([]string, 0, len(s))
			for _, item := range s {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}

func getBool(params map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := params[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func requireFloat64(params map[string]interface{}, key string) (float64, error) {
	if v, ok := params[key]; ok {
		if f, ok := v.(float64); ok {
			return f, nil
		}
		if f, ok := v.(int); ok {
			return float64(f), nil
		}
		return 0, fmt.Errorf("param %s is not a float64, got %T", key, v)
	}
	return 0, fmt.Errorf("param %s not found", key)
}

func validateIntRange(value int, min, max int, name string) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d, got %d", name, min, max, value)
	}
	return nil
}

func validateFloatRange(value float64, min, max float64, name string) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %.4f and %.4f, got %.4f", name, min, max, value)
	}
	return nil
}

func validatePositiveInt(value int, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive, got %d", name, value)
	}
	return nil
}

func validatePositiveFloat(value float64, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive, got %.4f", name, value)
	}
	return nil
}
