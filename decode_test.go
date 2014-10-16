// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"reflect"
	"strings"
	"testing"
)

type atype struct {
	Int       int
	String    string
	Ingore    string `json:"-"`
	Float     float64
	Bool      bool
	Exported  string
	exported  string
	Interface interface{}
}

var decodeTests = []struct {
	in  string
	ptr interface{}
	out interface{}
	err error
}{
	{in: `"hello"`, ptr: new(string), out: "hello"},
	{in: `"hello"`, ptr: new(interface{}), out: "hello"},
	{in: `123`, ptr: new(int), out: int(123)},
	{in: `123`, ptr: new(uint), out: uint(123)},
	{in: `456.789`, ptr: new(float64), out: float64(456.789)},
	{in: `987.654`, ptr: new(interface{}), out: float64(987.654)},
	{in: `[321]`, ptr: new(interface{}), out: []interface{}{float64(321)}},
	{in: `true`, ptr: new(bool), out: true},
	{in: `true`, ptr: new(interface{}), out: true},
	{in: "null", ptr: new(interface{}), out: nil},
	{in: `[1, 2, 3]`, ptr: new([3]int), out: [3]int{1, 2, 3}},
	{in: `[1, 2, 3]`, ptr: new([1]int), out: [1]int{1}},
	{in: `[1, 2, 3]`, ptr: new([5]int), out: [5]int{1, 2, 3, 0, 0}},
	{in: `[1, 2, 3]`, ptr: new([]int), out: []int{1, 2, 3}},
	{in: `[]`, ptr: new([]interface{}), out: []interface{}{}},
	{in: `null`, ptr: new([]interface{}), out: []interface{}(nil)},
	{in: `{"T":[]}`, ptr: new(map[string]interface{}), out: map[string]interface{}{"T": []interface{}{}}},
	{in: `{"T":null}`, ptr: new(map[string]interface{}), out: map[string]interface{}{"T": interface{}(nil)}},
	{in: `1`, ptr: new(atype), err: &DecodeTypeError{Type: reflect.TypeOf(atype{}), Kind: Number}},
	{in: `{"int": 123}`, ptr: new(atype), out: atype{Int: 123}},
	{in: `{"string": "hello"}`, ptr: new(atype), out: atype{String: "hello"}},
	{in: `{"ignore": "hello"}`, ptr: new(atype), out: atype{}},
	{in: `{"float": 123}`, ptr: new(atype), out: atype{Float: 123}},
	{in: `{"bool": true}`, ptr: new(atype), out: atype{Bool: true}},
	{in: `{"bool": false}`, ptr: new(atype), out: atype{Bool: false}},
	{in: `{"exported": "foo"}`, ptr: new(atype), out: atype{Exported: "foo"}},
	{in: `{"interface": "bar"}`, ptr: new(atype), out: atype{Interface: "bar"}},
	{in: `{"interface": [1]}`, ptr: new(atype), out: atype{Interface: []interface{}{float64(1)}}},
}

func TestDecode(t *testing.T) {
	for i, tt := range decodeTests {
		s := NewScanner(strings.NewReader(tt.in))
		v := reflect.New(reflect.TypeOf(tt.ptr).Elem())
		err := Unmarshal(s, v.Interface())
		var gotErr, wantErr string
		if err != nil {
			gotErr = err.Error()
		}
		if tt.err != nil {
			wantErr = tt.err.Error()
		}
		if gotErr != wantErr {
			t.Errorf("%d: err=%q, want %q", i, gotErr, wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if !reflect.DeepEqual(v.Elem().Interface(), tt.out) {
			t.Errorf("%d: value=%#+v, want %#+v", i, v.Elem().Interface(), tt.out)
			continue
		}
	}
}
