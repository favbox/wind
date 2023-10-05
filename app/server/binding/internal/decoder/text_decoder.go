package decoder

import (
	"fmt"
	"reflect"
	"strconv"

	wjson "github.com/favbox/wind/common/json"
	"github.com/favbox/wind/internal/bytesconv"
)

// TextDecoder 文本解码器，用于将文本解码到反射值。
type TextDecoder interface {
	UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error
}

// SelectTextDecoder 根据反射类型选择文本值的解码器。
func SelectTextDecoder(rt reflect.Type) (TextDecoder, error) {
	switch rt.Kind() {
	case reflect.Bool:
		return &boolDecoder{}, nil
	case reflect.Uint8:
		return &uintDecoder{bitSize: 8}, nil
	case reflect.Uint16:
		return &uintDecoder{bitSize: 16}, nil
	case reflect.Uint32:
		return &uintDecoder{bitSize: 32}, nil
	case reflect.Uint64:
		return &uintDecoder{bitSize: 64}, nil
	case reflect.Uint:
		return &uintDecoder{}, nil
	case reflect.Int8:
		return &intDecoder{bitSize: 8}, nil
	case reflect.Int16:
		return &intDecoder{bitSize: 16}, nil
	case reflect.Int32:
		return &intDecoder{bitSize: 32}, nil
	case reflect.Int64:
		return &intDecoder{bitSize: 64}, nil
	case reflect.Int:
		return &intDecoder{}, nil
	case reflect.Float32:
		return &floatDecoder{bitSize: 32}, nil
	case reflect.Float64:
		return &floatDecoder{bitSize: 64}, nil
	case reflect.String:
		return &stringDecoder{}, nil
	case reflect.Interface:
		return &interfaceDecoder{}, nil
	}

	return nil, fmt.Errorf("不支持的类型 %s", rt.String())
}

type boolDecoder struct{}

func (d *boolDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "false"
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	fieldValue.SetBool(v)
	return nil
}

type uintDecoder struct {
	bitSize int
}

func (d *uintDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	v, err := strconv.ParseUint(s, 10, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetUint(v)
	return nil
}

type intDecoder struct {
	bitSize int
}

func (d *intDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	v, err := strconv.ParseInt(s, 10, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetInt(v)
	return nil
}

type floatDecoder struct {
	bitSize int
}

func (d *floatDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	v, err := strconv.ParseFloat(s, d.bitSize)
	if err != nil {
		return err
	}
	fieldValue.SetFloat(v)
	return nil
}

type stringDecoder struct{}

func (d *stringDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	fieldValue.SetString(s)
	return nil
}

type interfaceDecoder struct{}

func (d *interfaceDecoder) UnmarshalString(s string, fieldValue reflect.Value, looseZeroMode bool) error {
	if s == "" && looseZeroMode {
		s = "0"
	}
	return wjson.Unmarshal(bytesconv.S2b(s), fieldValue.Addr().Interface())
}
