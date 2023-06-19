// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package deviceinfo

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Test ReadDeviceinfo and the logic of reading from multiple files
func TestReadDeviceinfo(t *testing.T) {
	compression_expected := "gz -9"
	mesa_expected := "msm"

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
	if devinfo.MesaDriver != mesa_expected {
		t.Errorf("expected %q, got: %q", mesa_expected, devinfo.MesaDriver)
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
		field    string
		in       string
		expected string
	}{
		{"InitfsCompression", "deviceinfo_initfs_compression=\"gzip:-9\"\n", "gzip:-9"},
		// line with multiple '='
		{"InitfsCompression", "deviceinfo_initfs_compression=zstd:--foo=1 -T0 --bar=bazz", "zstd:--foo=1 -T0 --bar=bazz"},
		// empty option
		{"InitfsCompression", "deviceinfo_initfs_compression=\"\"\n", ""},
		// line with comment at the end
		{"MesaDriver", "deviceinfo_mesa_driver=\"panfrost\"  # this is a nice driver", "panfrost"},
		{"", "# this is a comment!\n", ""},
		// empty lines are fine
		{"", "", ""},
		// line with whitepace characters only
		{"", " \t \n\r", ""},
	}
	var d DeviceInfo
	for _, table := range tables {
		testName := fmt.Sprintf("unmarshal::'%s':", strings.ReplaceAll(table.in, "\n", "\\n"))
		if err := d.unmarshal(strings.NewReader(table.in)); err != nil {
			t.Errorf("%s received an unexpected err: ", err)
		}

		// Check against expected value
		field := reflect.ValueOf(&d).Elem().FieldByName(table.field)
		out := ""
		if table.field != "" {
			out = field.String()
		}
		if out != table.expected {
			t.Errorf("%s expected: %q, got: %q", testName, table.expected, out)
		}
	}

}
