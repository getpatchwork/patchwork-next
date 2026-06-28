// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"reflect"
	"strconv"
	"strings"
)

type apiVersion struct {
	Major, Minor int
}

func parseVersion(s string) apiVersion {
	parts := strings.SplitN(s, ".", 2)
	v := apiVersion{}
	if len(parts) >= 1 {
		v.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		v.Minor, _ = strconv.Atoi(parts[1])
	}
	return v
}

func (v apiVersion) Before(other apiVersion) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	return v.Minor < other.Minor
}

// stripVersionedFields zeroes out any struct field tagged with
// `since:"X.Y"` when the requested API version is older than X.Y.
// Combined with `json:",omitempty"`, this makes the field disappear
// from the JSON output.
//
// When the input is a non-pointer struct (not addressable), this
// allocates a pointer copy so fields can be zeroed.
func stripVersionedFields(v any, ver apiVersion) any {
	rv := reflect.ValueOf(v)

	// If the value is a struct but not addressable, allocate a
	// pointer copy so we can modify fields in place.
	if rv.Kind() == reflect.Struct {
		p := reflect.New(rv.Type())
		p.Elem().Set(rv)
		rv = p
	}

	stripValue(rv, ver)
	return rv.Interface()
}

func stripValue(v reflect.Value, ver apiVersion) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		stripStruct(v, ver)
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			stripValue(v.Index(i), ver)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			stripValue(v.MapIndex(key), ver)
		}
	}
}

func stripStruct(v reflect.Value, ver apiVersion) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fv := v.Field(i)

		if f.Anonymous {
			stripValue(fv, ver)
			continue
		}

		since := f.Tag.Get("since")
		if since != "" && ver.Before(parseVersion(since)) {
			fv.Set(reflect.Zero(f.Type))
			continue
		}

		stripValue(fv, ver)
	}
}
