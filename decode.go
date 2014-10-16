// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"encoding/base64"
	"errors"
	"reflect"
	"strconv"
)

// Unmarshal deserializes data from the scanner to value v. In the case of
// struct values, only exported fields will be decoded. The lowercase field
// name is used as the key for each exported field, but this behavior may be
// changed using the respective field tag. The tag may also contain flags to
// tweak the decoding behavior for the field.
func Unmarshal(s *Scanner, v interface{}) error {
	value, ok := v.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(v)
		switch value.Kind() {
		case reflect.Map:
			if value.IsNil() {
				return errors.New("map arg must not be nil")
			}
		case reflect.Ptr:
			if value.IsNil() {
				return errors.New("pointer arg must not be nil")
			}
			value = value.Elem()
		default:
			return errors.New("arg must be pointer or map")
		}
	}

	d := decoder{s}
	if !d.s.Scan() {
		return d.s.Err()
	}

	if err := d.decode(value); err != nil {
		return err
	}

	d.s.Scan()
	return d.s.Err()
}

type decoder struct {
	s *Scanner
}

// A DecodeTypeError describes a JSON value that was not appropriate for a value of a specific Go type.
type DecodeTypeError struct {
	Kind Kind         // description of JSON value
	Type reflect.Type // type of Go value it could not be assigned to
}

func (e *DecodeTypeError) Error() string {
	return "cannot unmarshal " + e.Kind.String() + " into Go value of type " + e.Type.String()
}

func (d *decoder) typeError(v reflect.Value) error {
	err := &DecodeTypeError{
		Kind: d.s.Kind(),
		Type: v.Type(),
	}
	return err
}

func (d *decoder) decode(v reflect.Value) error {
	if d.s.Kind() == Null {
		v.Set(reflect.Zero(v.Type()))
		return nil
	}

	v = indirect(v)
	typ := v.Type()
	decoder, ok := typeDecoder[typ]
	if !ok {
		decoder, ok = kindDecoder[typ.Kind()]
		if !ok {
			return d.typeError(v)
		}
	}
	return decoder(d, v)
}

// indirect walks down v allocating pointers as needed, until it gets to a
// non-pointer.
func indirect(v reflect.Value) reflect.Value {
	for {
		if v.Kind() == reflect.Interface && !v.IsNil() {
			v = v.Elem()
			continue
		}
		if v.Kind() != reflect.Ptr {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

func (d *decoder) decodeFloat(v reflect.Value) error {
	if d.s.Kind() != Number {
		return d.typeError(v)
	}
	f, err := strconv.ParseFloat(string(d.s.Value()), v.Type().Bits())
	if err != nil {
		return d.typeError(v)
	}
	v.SetFloat(f)
	return nil
}

func (d *decoder) decodeInt(v reflect.Value) error {
	if d.s.Kind() != Number {
		return d.typeError(v)
	}
	i, err := strconv.ParseInt(string(d.s.Value()), 10, v.Type().Bits())
	if err != nil {
		return d.typeError(v)
	}
	v.SetInt(i)
	return nil
}

func (d *decoder) decodeUint(v reflect.Value) error {
	if d.s.Kind() != Number {
		return d.typeError(v)
	}
	u, err := strconv.ParseUint(string(d.s.Value()), 10, v.Type().Bits())
	if err != nil {
		return d.typeError(v)
	}
	v.SetUint(u)
	return nil
}

func (d *decoder) decodeString(v reflect.Value) error {
	if d.s.Kind() != String {
		return d.typeError(v)
	}
	v.SetString(string(d.s.Value()))
	return nil
}

func (d *decoder) decodeByteSlice(v reflect.Value) error {
	if d.s.Kind() != String {
		return d.typeError(v)
	}
	p := d.s.Value()
	n := base64.StdEncoding.DecodedLen(len(p))
	if v.IsNil() || v.Cap() < n {
		v.Set(reflect.MakeSlice(v.Type(), n, n))
	} else {
		v.SetLen(n)
	}
	n, err := base64.StdEncoding.Decode(v.Interface().([]byte), p)
	if err != nil {
		return d.typeError(v)
	}
	v.SetLen(n)
	return nil
}

func (d *decoder) decodeBool(v reflect.Value) error {
	if d.s.Kind() != Bool {
		return d.typeError(v)
	}
	v.SetBool(d.s.BoolValue())
	return nil
}

func (d *decoder) decodeMapStringInterface(v reflect.Value) error {
	if d.s.Kind() != Object {
		return d.typeError(v)
	}

	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	m := v.Interface().(map[string]interface{})
	var savedErr error
	os := d.s.ObjectScanner()
	for os.Scan() {
		v, err := d.decodeValueInterface()
		if err == nil {
			m[os.Name()] = v
		} else if savedErr == nil {
			savedErr = err
		}
	}
	if err := d.s.Err(); err != nil {
		return err
	}
	return savedErr
}

func (d *decoder) decodeMap(v reflect.Value) error {
	if d.s.Kind() != Object {
		return d.typeError(v)
	}

	typ := v.Type()
	if typ.Key().Kind() != reflect.String {
		return d.typeError(v)
	}

	if v.IsNil() {
		v.Set(reflect.MakeMap(typ))
	}

	subv := reflect.New(typ.Elem()).Elem()
	var savedErr error
	os := d.s.ObjectScanner()
	for os.Scan() {
		subv.Set(reflect.Zero(typ.Elem()))
		err := d.decode(subv)
		if err == nil {
			v.SetMapIndex(reflect.ValueOf(os.Name()), subv)
		} else if savedErr == nil {
			savedErr = err
		}
	}
	if err := d.s.Err(); err != nil {
		return err
	}
	return savedErr
}

func (d *decoder) decodeSlice(v reflect.Value) error {
	if d.s.Kind() != Array {
		return d.typeError(v)
	}

	typ := v.Type()
	if v.IsNil() {
		v.Set(reflect.MakeSlice(typ, 0, 0))
	}

	i := 0
	var savedErr error
	as := d.s.ArrayScanner()
	for as.Scan() {
		if i >= v.Cap() {
			newcap := v.Cap() + v.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newv := reflect.MakeSlice(typ, v.Len(), newcap)
			reflect.Copy(newv, v)
			v.Set(newv)
		}
		v.SetLen(i + 1)
		err := d.decode(v.Index(i))
		if err != nil && savedErr == nil {
			savedErr = err
		}
		i += 1
	}
	if err := d.s.Err(); err != nil {
		return err
	}
	return savedErr
}

func (d *decoder) decodeArray(v reflect.Value) error {
	if d.s.Kind() != Array {
		return d.typeError(v)
	}

	i := 0
	var savedErr error
	as := d.s.ArrayScanner()
	for as.Scan() {
		if i < v.Len() {
			err := d.decode(v.Index(i))
			if err != nil && savedErr == nil {
				savedErr = err
			}
		}
		i += 1
	}
	if err := d.s.Err(); err != nil {
		return err
	}
	return savedErr
}

func (d *decoder) decodeStruct(v reflect.Value) error {
	if d.s.Kind() != Object {
		return d.typeError(v)
	}

	var savedErr error
	typ := v.Type()
	ss := structSpecForType(typ)
	os := d.s.ObjectScanner()
	for os.Scan() {
		if fs := ss.m[os.Name()]; fs != nil {
			err := d.decode(v.FieldByIndex(fs.index))
			if err != nil && savedErr == nil {
				savedErr = err
			}
		}
	}
	if err := d.s.Err(); err != nil {
		return err
	}
	return savedErr
}

func (d *decoder) decodeInterface(v reflect.Value) error {
	x, err := d.decodeValueInterface()
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(x))
	return nil
}

func (d *decoder) decodeValueInterface() (interface{}, error) {
	switch d.s.Kind() {
	case Number:
		return strconv.ParseFloat(string(d.s.Value()), 64)
	case String:
		return string(d.s.Value()), nil
	case Bool:
		return d.s.BoolValue(), nil
	case Null:
		return nil, nil
	case Object:
		var savedErr error
		m := make(map[string]interface{})
		os := d.s.ObjectScanner()
		for os.Scan() {
			v, err := d.decodeValueInterface()
			if err != nil && savedErr == nil {
				savedErr = err
			}
			m[os.Name()] = v
		}
		if err := d.s.Err(); err != nil {
			return nil, err
		}
		return m, savedErr
	case Array:
		var savedErr error
		a := make([]interface{}, 0)
		as := d.s.ArrayScanner()
		for as.Scan() {
			v, err := d.decodeValueInterface()
			if err != nil && savedErr == nil {
				savedErr = err
			}
			a = append(a, v)
		}
		if err := d.s.Err(); err != nil {
			return nil, err
		}
		return a, savedErr
	default:
		return nil, d.typeError(reflect.ValueOf(new(interface{})).Elem())
	}
}

type decoderFunc func(d *decoder, v reflect.Value) error

var kindDecoder map[reflect.Kind]decoderFunc
var typeDecoder map[reflect.Type]decoderFunc

func init() {
	kindDecoder = map[reflect.Kind]decoderFunc{
		reflect.Bool:    (*decoder).decodeBool,
		reflect.Float32: (*decoder).decodeFloat,
		reflect.Float64: (*decoder).decodeFloat,
		reflect.Int8:    (*decoder).decodeInt,
		reflect.Int16:   (*decoder).decodeInt,
		reflect.Int32:   (*decoder).decodeInt,
		reflect.Int64:   (*decoder).decodeInt,
		reflect.Int:     (*decoder).decodeInt,
		reflect.Uint8:   (*decoder).decodeUint,
		reflect.Uint16:  (*decoder).decodeUint,
		reflect.Uint32:  (*decoder).decodeUint,
		reflect.Uint64:  (*decoder).decodeUint,
		reflect.Uint:    (*decoder).decodeUint,
		reflect.Map:     (*decoder).decodeMap,
		reflect.String:  (*decoder).decodeString,
		reflect.Struct:  (*decoder).decodeStruct,
		reflect.Slice:   (*decoder).decodeSlice,
		reflect.Array:   (*decoder).decodeArray,
	}
	typeDecoder = map[reflect.Type]decoderFunc{
		reflect.TypeOf(make(map[string]interface{})): (*decoder).decodeMapStringInterface,
		reflect.TypeOf(new(interface{})).Elem():      (*decoder).decodeInterface,
	}
}
