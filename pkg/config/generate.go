// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

var fieldExamples = map[string][]string{
	"DatabaseConfig.URL": {
		"postgres://patchwork:patchwork@localhost/patchwork?sslmode=disable",
		"sqlite://patchwork.db",
		"mysql://patchwork:patchwork@localhost/patchwork",
	},
}

func Generate(cfg *Config, w io.Writer) error {
	v := reflect.ValueOf(*cfg)
	t := v.Type()

	fmt.Fprintln(w, "# Patchwork configuration file.")

	for i := range t.NumField() {
		field := t.Field(i)

		if _, ok := field.Tag.Lookup("embed"); ok {
			prefix := strings.TrimSuffix(field.Tag.Get("prefix"), "-")
			fmt.Fprintf(w, "\n[%s]\n", prefix)
			writeSection(w, field.Type, v.Field(i))
			continue
		}

		writeField(w, t.Name(), field, v.Field(i), "\n")
	}

	return nil
}

func writeSection(w io.Writer, t reflect.Type, v reflect.Value) {
	for i := range t.NumField() {
		writeField(w, t.Name(), t.Field(i), v.Field(i), "")
	}
}

func camelToKebab(s string) string {
	var buf strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			lower := c + ('a' - 'A')
			if i > 0 && s[i-1] >= 'a' && s[i-1] <= 'z' {
				buf.WriteByte('-')
			} else if i > 0 && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
				buf.WriteByte('-')
			}
			buf.WriteByte(lower)
		} else {
			buf.WriteByte(c)
		}
	}
	return buf.String()
}

func writeField(w io.Writer, typeName string, field reflect.StructField, val reflect.Value, prefix string) {
	name := field.Tag.Get("name")
	if name == "" {
		name = camelToKebab(field.Name)
	}

	help := field.Tag.Get("help")
	if help != "" {
		fmt.Fprintf(w, "%s# %s\n", prefix, help)
	} else if prefix != "" {
		fmt.Fprint(w, prefix)
	}

	examples := fieldExamples[typeName+"."+field.Name]
	active := ""

	if !val.IsZero() {
		active = fmt.Sprint(val.Interface())
	} else if def := field.Tag.Get("default"); def != "" {
		active = def
	}

	if len(examples) > 0 {
		if active != "" {
			fmt.Fprintf(w, "%s = %s\n", name, formatValue(field.Type, active))
		}
		for _, ex := range examples {
			if ex == active {
				continue
			}
			fmt.Fprintf(w, "#%s = %s\n", name, formatValue(field.Type, ex))
		}
		return
	}

	if active != "" {
		fmt.Fprintf(w, "%s = %s\n", name, formatValue(field.Type, active))
	} else {
		fmt.Fprintf(w, "#%s = %s\n", name, zeroValue(field.Type))
	}
}

func formatValue(t reflect.Type, val string) string {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int64:
		return val
	default:
		return fmt.Sprintf("%q", val)
	}
}

func zeroValue(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "false"
	case reflect.Int, reflect.Int64:
		return "0"
	default:
		return `""`
	}
}
