// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
	"reflect"
	"sort"
	"testing"
)

func TestMerge(t *testing.T) {
	subtests := []struct {
		name     string
		inA      map[string]string
		inB      map[string]string
		expected map[string]string
	}{
		{
			name: "empty B",
			inA: map[string]string{
				"foo":    "bar",
				"banana": "airplane",
			},
			inB: map[string]string{},
			expected: map[string]string{
				"foo":    "bar",
				"banana": "airplane",
			},
		},
		{
			name: "empty A",
			inA:  map[string]string{},
			inB: map[string]string{
				"foo":    "bar",
				"banana": "airplane",
			},
			expected: map[string]string{
				"foo":    "bar",
				"banana": "airplane",
			},
		},
		{
			name: "both populated, some duplicates",
			inA: map[string]string{
				"bar":    "bazz",
				"banana": "yellow",
				"guava":  "green",
			},
			inB: map[string]string{
				"foo":    "bar",
				"banana": "airplane",
			},
			expected: map[string]string{
				"foo":    "bar",
				"guava":  "green",
				"banana": "airplane",
				"bar":    "bazz",
			},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			out := st.inA
			Merge(out, st.inB)
			if !reflect.DeepEqual(st.expected, out) {
				t.Fatalf("expected: %q, got: %q\n", st.expected, out)
			}
		})
	}
}

func TestRemoveDuplicates(t *testing.T) {
	subtests := []struct {
		name     string
		in       []string
		expected []string
	}{
		{
			name: "no duplicates",
			in: []string{
				"foo",
				"bar",
				"banana",
				"airplane",
			},
			expected: []string{
				"foo",
				"bar",
				"banana",
				"airplane",
			},
		},
		{
			name: "all duplicates",
			in: []string{
				"foo",
				"foo",
				"foo",
				"foo",
			},
			expected: []string{
				"foo",
			},
		},
		{
			name:     "empty",
			in:       []string{},
			expected: []string{},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			// note: sorting to make comparison easier later
			sort.Strings(st.expected)
			out := RemoveDuplicates(st.in)
			sort.Strings(out)
			if !reflect.DeepEqual(st.expected, out) {
				t.Fatalf("expected: %q, got: %q\n", st.expected, out)
			}
		})
	}
}
