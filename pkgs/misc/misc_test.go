// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
	"reflect"
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
