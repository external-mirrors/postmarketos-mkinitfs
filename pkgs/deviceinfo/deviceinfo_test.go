// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package deviceinfo

import (
	"strings"
	"testing"
)

// Test ReadDeviceinfo and the logic of reading from multiple files
func TestReadDeviceinfo(t *testing.T) {
	compression_expected := "gz -9"

	var devinfo DeviceInfo
	err := devinfo.ReadDeviceinfo("./test_resources/deviceinfo-missing")
	if !strings.Contains(err.Error(), "required by mkinitfs") {
		t.Errorf("received an unexpected err: %s", err)
	}
	err = devinfo.ReadDeviceinfo("./test_resources/deviceinfo-first")
	if err != nil {
		t.Errorf("received an unexpected err: %s", err)
	}
	err = devinfo.ReadDeviceinfo("./test_resources/deviceinfo-msm")
	if err != nil {
		t.Errorf("received an unexpected err: %s", err)
	}
	if devinfo.InitfsCompression != compression_expected {
		t.Errorf("expected %q, got: %q", compression_expected, devinfo.InitfsCompression)
	}
}

// Test conversion of name to DeviceInfo struct field format
func TestNameToField(t *testing.T) {
	tables := []struct {
		in       string
		expected string
	}{
		{"deviceinfo_dtb", "Dtb"},
		{"dtb", "Dtb"},
		{"deviceinfo_initfs_compression", "InitfsCompression"},
		{"modules_initfs", "ModulesInitfs"},
		{"deviceinfo_initfs_compression___", "InitfsCompression"},
		{"deviceinfo_initfs_extra_compression", "InitfsExtraCompression"},
		{"deviceinfo_create_initfs_extra", "CreateInitfsExtra"},
	}

	for _, table := range tables {
		out := nameToField(table.in)
		if out != table.expected {
			t.Errorf("expected: %q, got: %q", table.expected, out)
		}
	}
}

// Test unmarshalling with lines in deviceinfo
func TestUnmarshal(t *testing.T) {
	tables := []struct {
		// field is just used for reflection within the test, so it must be a
		// valid DeviceInfo field
		file     string
		expected DeviceInfo
	}{
		{"./test_resources/deviceinfo-unmarshal-1", DeviceInfo{
			FormatVersion:          "0",
			UbootBoardname:         "foobar-bazz",
			InitfsCompression:      "zstd:--foo=1 -T0 --bar=bazz",
			InitfsExtraCompression: "",
			CreateInitfsExtra:      true,
		},
		},
	}
	var d DeviceInfo
	for _, table := range tables {
		if err := d.unmarshal(table.file); err != nil {
			t.Error(err)
		}
		if d != table.expected {
			t.Errorf("expected: %s, got: %s", table.expected, d)
		}
	}

}
