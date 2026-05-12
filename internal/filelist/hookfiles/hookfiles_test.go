// Copyright 2026 Aster Boese <asterboese@mailbox.org>
// SPDX-License-Identifier: GPL-3.0-or-later

package hookfiles

import (
	"testing"
)

func TestStripSuffix(t *testing.T) {
	tables := []struct {
		in                   string
		expected_src         string
		expected_dest        string
		expected_has_dest    bool
		expected_is_optional bool
	}{
		{"/foo/bar/bazz", "/foo/bar/bazz", "", false, false},
		{"/foo/bar/bazz:/foo/bazz", "/foo/bar/bazz", "/foo/bazz", true, false},
		{"/lentil/soup!optional", "/lentil/soup", "", false, true},
		{"/lentil/soup:/carrot/soup!optional", "/lentil/soup", "/carrot/soup", true, true},
	}
	for _, table := range tables {
		real_src, real_dest, real_has_dest, real_is_optional := stripSuffix(table.in)
		if real_src != table.expected_src {
			t.Errorf("Expected: %q, got: %q", table.expected_src, real_src)
		}
		if real_dest != table.expected_dest {
			t.Errorf("Expected: %q, got: %q", table.expected_dest, real_dest)
		}
		if real_has_dest != table.expected_has_dest {
			t.Errorf("Expected: %t, got: %t", table.expected_has_dest, real_has_dest)
		}
		if real_is_optional != table.expected_is_optional {
			t.Errorf("Expected: %t, got: %t", table.expected_is_optional, real_is_optional)
		}
	}
}
