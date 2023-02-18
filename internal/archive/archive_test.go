// Copyright 2022 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package archive

import (
	"reflect"
	"testing"

	"github.com/cavaliercoder/go-cpio"
)

func TestArchiveItemsAdd(t *testing.T) {
	subtests := []struct {
		name     string
		inItems  []archiveItem
		inItem   archiveItem
		expected []archiveItem
	}{
		{
			name:    "empty list",
			inItems: []archiveItem{},
			inItem: archiveItem{
				sourcePath: "/foo/bar",
				header:     &cpio.Header{Name: "/foo/bar"},
			},
			expected: []archiveItem{
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
			},
		},
		{
			name: "already exists",
			inItems: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
			},
			inItem: archiveItem{
				sourcePath: "/foo",
				header:     &cpio.Header{Name: "/foo"},
			},
			expected: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
			},
		},
		{
			name: "add new",
			inItems: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
				{
					sourcePath: "/foo/bar1",
					header:     &cpio.Header{Name: "/foo/bar1"},
				},
			},
			inItem: archiveItem{
				sourcePath: "/foo/bar0",
				header:     &cpio.Header{Name: "/foo/bar0"},
			},
			expected: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
				{
					sourcePath: "/foo/bar0",
					header:     &cpio.Header{Name: "/foo/bar0"},
				},
				{
					sourcePath: "/foo/bar1",
					header:     &cpio.Header{Name: "/foo/bar1"},
				},
			},
		},
		{
			name: "add new at beginning",
			inItems: []archiveItem{
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
			},
			inItem: archiveItem{
				sourcePath: "/bazz/bar",
				header:     &cpio.Header{Name: "/bazz/bar"},
			},
			expected: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/foo/bar",
					header:     &cpio.Header{Name: "/foo/bar"},
				},
			},
		},
		{
			name: "add new at end",
			inItems: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
			},
			inItem: archiveItem{
				sourcePath: "/zzz/bazz",
				header:     &cpio.Header{Name: "/zzz/bazz"},
			},
			expected: []archiveItem{
				{
					sourcePath: "/bazz/bar",
					header:     &cpio.Header{Name: "/bazz/bar"},
				},
				{
					sourcePath: "/foo",
					header:     &cpio.Header{Name: "/foo"},
				},
				{
					sourcePath: "/zzz/bazz",
					header:     &cpio.Header{Name: "/zzz/bazz"},
				},
			},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			a := archiveItems{items: st.inItems}
			a.add(st.inItem)
			if !reflect.DeepEqual(st.expected, a.items) {
				t.Fatal("expected:", st.expected, " got: ", a.items)
			}
		})
	}
}
