// Copyright 2024 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package osutil

import (
	"testing"
)

func TestMergeUsr(t *testing.T) {
	subtests := []struct {
		in       string
		expected string
	}{
		{
			in:       "/bin/foo",
			expected: "/usr/bin/foo",
		},
		{
			in:       "/sbin/foo",
			expected: "/usr/bin/foo",
		},
		{
			in:       "/usr/sbin/foo",
			expected: "/usr/bin/foo",
		},
		{
			in:       "/usr/bin/foo",
			expected: "/usr/bin/foo",
		},
		{
			in:       "/lib/foo.so",
			expected: "/usr/lib/foo.so",
		},
		{
			in:       "/lib64/foo.so",
			expected: "/usr/lib64/foo.so",
		},
	}

	for _, st := range subtests {
		t.Run(st.in, func(t *testing.T) {
			out := MergeUsr(st.in)
			if out != st.expected {
				t.Fatalf("expected: %q, got: %q\n", st.expected, out)
			}
		})
	}
}
