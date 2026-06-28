// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"encoding/json"
	"testing"
)

func TestStripVersionedFields(t *testing.T) {
	type inner struct {
		URL    string  `json:"url"`
		WebURL *string `json:"web_url,omitempty" since:"1.1"`
	}

	type resp struct {
		ID               int     `json:"id"`
		Name             string  `json:"name"`
		SubjectMatch     *string `json:"subject_match,omitempty" since:"1.1"`
		ListArchiveURL   *string `json:"list_archive_url,omitempty" since:"1.2"`
		ShowDependencies *bool   `json:"show_dependencies,omitempty" since:"1.4"`
		Nested           inner   `json:"nested"`
	}

	s := func(v string) *string { return &v }
	b := func(v bool) *bool { return &v }

	obj := resp{
		ID:               1,
		Name:             "test",
		SubjectMatch:     s(".*"),
		ListArchiveURL:   s("https://example.com"),
		ShowDependencies: b(true),
		Nested: inner{
			URL:    "https://api/projects/1/",
			WebURL: s("https://web/projects/1/"),
		},
	}

	tests := []struct {
		version string
		want    string
	}{
		{
			version: "1.0",
			want:    `{"id":1,"name":"test","nested":{"url":"https://api/projects/1/"}}`,
		},
		{
			version: "1.1",
			want:    `{"id":1,"name":"test","subject_match":".*","nested":{"url":"https://api/projects/1/","web_url":"https://web/projects/1/"}}`,
		},
		{
			version: "1.2",
			want:    `{"id":1,"name":"test","subject_match":".*","list_archive_url":"https://example.com","nested":{"url":"https://api/projects/1/","web_url":"https://web/projects/1/"}}`,
		},
		{
			version: "1.4",
			want:    `{"id":1,"name":"test","subject_match":".*","list_archive_url":"https://example.com","show_dependencies":true,"nested":{"url":"https://api/projects/1/","web_url":"https://web/projects/1/"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			// work on a copy
			o := obj
			o.Nested = obj.Nested
			result := stripVersionedFields(o, parseVersion(tt.version))

			got, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("version %s:\n got: %s\nwant: %s",
					tt.version, got, tt.want)
			}
		})
	}
}

func TestStripVersionedFieldsSlice(t *testing.T) {
	type item struct {
		ID     int     `json:"id"`
		WebURL *string `json:"web_url,omitempty" since:"1.1"`
	}

	items := []item{
		{ID: 1, WebURL: ptr("https://a/")},
		{ID: 2, WebURL: ptr("https://b/")},
	}

	result := stripVersionedFields(items, parseVersion("1.0"))
	items = result.([]item)

	for i, it := range items {
		if it.WebURL != nil {
			t.Errorf("items[%d].WebURL = %q, want nil", i, *it.WebURL)
		}
	}
}

func ptr(s string) *string { return &s }
