// Copyright 2021 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package copier

import (
	"encoding"
	"github.com/juju/errors"
	"reflect"
)

func Copy(dst, src interface{}) error {
	dstPtr := reflect.ValueOf(dst)
	if dstPtr.Kind() != reflect.Ptr {
		return errors.NotValidf("expect dst to be a pointer, but receive %v", dstPtr.Kind())
	}

	dstValue := dstPtr.Elem()
	srcValue := reflect.ValueOf(src)
	return copyValue(dstValue, srcValue)
}

func copyValue(dst, src reflect.Value) error {
	if dst.Kind() != src.Kind() {
		if dst.Kind() == reflect.Interface {
			newValuePointer := reflect.New(src.Type())
			err := copyValue(newValuePointer.Elem(), src)
			if err != nil {
				return err
			}
			dst.Set(newValuePointer.Elem())
			return nil
		} else {
			return errors.NotValidf("different type: %v != %v", dst.Kind(), src.Kind())
		}
	}

	switch dst.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.String:
		dst.Set(src)
	case reflect.Slice:
		if dst.IsNil() || (!dst.CanAddr() && dst.Len() != src.Len()) || dst.Cap() < src.Len() {
			newSlice := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
			dst.Set(newSlice)
		} else if dst.CanAddr() {
			dst.SetLen(src.Len())
		}
		for i := 0; i < src.Len(); i++ {
			err := copyValue(dst.Index(i), src.Index(i))
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		if !reflect.DeepEqual(dst.Interface(), src.Interface()) {
			dst.Set(reflect.MakeMap(dst.Type()))
			keys := src.MapKeys()
			for _, k := range keys {
				value := src.MapIndex(k)
				newValuePointer := reflect.New((value).Type())
				err := copyValue(newValuePointer.Elem(), src.MapIndex(k))
				if err != nil {
					return err
				}
				dst.SetMapIndex(k, newValuePointer.Elem())
			}
		}
	case reflect.Struct:
		if dst.Type().Name() != src.Type().Name() {
			return errors.NotValidf("different struct: %v != %v", dst.Type().Name(), src.Type().Name())
		}

		dstPointer := reflect.New(dst.Type())
		srcPointer := reflect.New(src.Type())
		srcPointer.Elem().Set(src)
		srcMarshaller, hasSrcMarshaler := srcPointer.Interface().(encoding.BinaryMarshaler)
		dstUnmarshaler, hasDstUnmarshaler := dstPointer.Interface().(encoding.BinaryUnmarshaler)

		if hasDstUnmarshaler && hasSrcMarshaler {
			dstByte, err := srcMarshaller.MarshalBinary()
			if err != nil {
				return err
			}
			err = dstUnmarshaler.UnmarshalBinary(dstByte)
			if err != nil {
				return err
			}
			dst.Set(dstPointer.Elem())
		} else {
			numFiled := src.NumField()
			for i := 0; i < numFiled; i++ {
				fieldDST := dst.Field(i)
				fieldSRC := src.Field(i)
				if !fieldDST.CanSet() {
					continue
				}
				err := copyValue(fieldDST, fieldSRC)
				if err != nil {
					return err
				}
			}
		}
	case reflect.Ptr:
		if dst.IsNil() {
			dst.Set(reflect.New(src.Elem().Type()))
		}
		srcElem := src.Elem()
		dstElem := dst.Elem()
		err := copyValue(dstElem, srcElem)
		if err != nil {
			return err
		}
	case reflect.Interface:
		if !dst.IsNil() {
			switch dst.Elem().Kind() {
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
				reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64,
				reflect.Complex64, reflect.Complex128, reflect.String:
				newValuePointer := reflect.New(src.Elem().Type())
				err := copyValue(newValuePointer.Elem(), src.Elem())
				if err != nil {
					return err
				}
				dst.Set(newValuePointer.Elem())
			default:
				err := copyValue(dst.Elem(), src.Elem())
				if err != nil {
					return err
				}
			}
		} else {
			newValuePointer := reflect.New(src.Elem().Type())
			err := copyValue(newValuePointer.Elem(), src.Elem())
			if err != nil {
				return err
			}
			dst.Set(newValuePointer.Elem())
		}
	default:
		return errors.NotValidf("unsupported type: %v", dst.Kind())
	}
	return nil
}
