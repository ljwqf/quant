package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFloat64(t *testing.T) {
	params := map[string]interface{}{
		"float_value": 10.5,
		"int_value":   20,
	}

	// 测试正常浮点值
	value := getFloat64(params, "float_value", 0.0)
	assert.Equal(t, 10.5, value)

	// 测试整数转换为浮点
	value = getFloat64(params, "int_value", 0.0)
	assert.Equal(t, 20.0, value)

	// 测试不存在的键
	value = getFloat64(params, "non_existent", 0.0)
	assert.Equal(t, 0.0, value)

	// 测试类型错误
	params["invalid_type"] = true
	value = getFloat64(params, "invalid_type", 0.0)
	assert.Equal(t, 0.0, value)
}

func TestGetInt(t *testing.T) {
	params := map[string]interface{}{
		"int_value":    42,
		"float_value":  42.5,
	}

	// 测试正常整数值
	value := getInt(params, "int_value", 0)
	assert.Equal(t, 42, value)

	// 测试浮点数转换为整数（向下取整）
	value = getInt(params, "float_value", 0)
	assert.Equal(t, 42, value)

	// 测试不存在的键
	value = getInt(params, "non_existent", 0)
	assert.Equal(t, 0, value)

	// 测试类型错误
	params["invalid_type"] = true
	value = getInt(params, "invalid_type", 0)
	assert.Equal(t, 0, value)
}

func TestGetString(t *testing.T) {
	params := map[string]interface{}{
		"string_value": "test",
	}

	// 测试正常字符串值
	value := getString(params, "string_value", "")
	assert.Equal(t, "test", value)

	// 测试不存在的键
	value = getString(params, "non_existent", "")
	assert.Equal(t, "", value)

	// 测试类型错误
	params["invalid_type"] = true
	value = getString(params, "invalid_type", "")
	assert.Equal(t, "", value)
}

func TestGetStringSlice(t *testing.T) {
	params := map[string]interface{}{
		"string_slice": []string{"a", "b", "c"},
	}

	// 测试正常字符串切片
	value := getStringSlice(params, "string_slice", []string{})
	assert.Equal(t, []string{"a", "b", "c"}, value)

	// 测试不存在的键
	value = getStringSlice(params, "non_existent", []string{"default"})
	assert.Equal(t, []string{"default"}, value)
}

func TestGetBool(t *testing.T) {
	params := map[string]interface{}{
		"bool_value": true,
	}

	// 测试正常布尔值
	value := getBool(params, "bool_value", false)
	assert.Equal(t, true, value)

	// 测试不存在的键
	value = getBool(params, "non_existent", false)
	assert.Equal(t, false, value)

	// 测试类型错误
	params["invalid_type"] = 123
	value = getBool(params, "invalid_type", false)
	assert.Equal(t, false, value)
}

func TestRequireFloat64(t *testing.T) {
	params := map[string]interface{}{
		"float_value": 10.5,
		"int_value":   20,
	}

	// 测试正常浮点值
	value, err := requireFloat64(params, "float_value")
	assert.NoError(t, err)
	assert.Equal(t, 10.5, value)

	// 测试整数转换为浮点
	value, err = requireFloat64(params, "int_value")
	assert.NoError(t, err)
	assert.Equal(t, 20.0, value)

	// 测试不存在的键
	_, err = requireFloat64(params, "non_existent")
	assert.Error(t, err)

	// 测试类型错误
	params["invalid_type"] = true
	_, err = requireFloat64(params, "invalid_type")
	assert.Error(t, err)
}
