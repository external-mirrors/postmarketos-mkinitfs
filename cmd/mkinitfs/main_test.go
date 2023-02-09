// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

func TestStripExts(t *testing.T) {
	tables := []struct {
		in       string
		expected string
	}{
		{"/foo/bar/bazz.tar", "/foo/bar/bazz"},
		{"file.tar.gz.xz.zip", "file"},
		{"another_file", "another_file"},
		{"a.b.c.d.e.f.g.h.i", "a"},
		{"virtio_blk.ko", "virtio_blk"},
	}
	for _, table := range tables {
		out := stripExts(table.in)
		if out != table.expected {
			t.Errorf("Expected: %q, got: %q", table.expected, out)
		}
	}
}

func stringSlicesEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

var testModuleDep string = `
kernel/sound/soc/codecs/snd-soc-msm8916-digital.ko:
kernel/net/sched/act_ipt.ko.xz: kernel/net/netfilter/x_tables.ko.xz
kernel/drivers/watchdog/watchdog.ko.xz:
kernel/drivers/usb/serial/ir-usb.ko.xz: kernel/drivers/usb/serial/usbserial.ko.xz
kernel/drivers/gpu/drm/scheduler/gpu-sched.ko.xz:
kernel/drivers/hid/hid-alps.ko.xz:
kernel/net/netfilter/xt_u32.ko.xz: kernel/net/netfilter/x_tables.ko.xz
kernel/net/netfilter/xt_sctp.ko.xz: kernel/net/netfilter/x_tables.ko.xz
kernel/drivers/hwmon/gl518sm.ko.xz:
kernel/drivers/watchdog/dw_wdt.ko.xz: kernel/drivers/watchdog/watchdog.ko.xz
kernel/net/bluetooth/hidp/hidp.ko.xz: kernel/net/bluetooth/bluetooth.ko.xz kernel/net/rfkill/rfkill.ko.xz kernel/crypto/ecdh_generic.ko.xz kernel/crypto/ecc.ko.xz
kernel/fs/nls/nls_iso8859-1.ko.xz:
kernel/net/vmw_vsock/vmw_vsock_virtio_transport.ko.xz: kernel/net/vmw_vsock/vmw_vsock_virtio_transport_common.ko.xz kernel/drivers/virtio/virtio.ko.xz kernel/drivers/virtio/virtio_ring.ko.xz kernel/net/vmw_vsock/vsock.ko.xz
kernel/drivers/gpu/drm/panfrost/panfrost.ko.xz: kernel/drivers/gpu/drm/scheduler/gpu-sched.ko.xz
kernel/drivers/gpu/drm/msm/msm.ko: kernel/drivers/gpu/drm/drm_kms_helper.ko
`

func TestGetModuleDeps(t *testing.T) {
	tables := []struct {
		in       string
		expected []string
	}{
		{"nls-iso8859-1", []string{"kernel/fs/nls/nls_iso8859-1.ko.xz"}},
		{"gpu_sched", []string{"kernel/drivers/gpu/drm/scheduler/gpu-sched.ko.xz"}},
		{"dw-wdt", []string{"kernel/drivers/watchdog/dw_wdt.ko.xz",
			"kernel/drivers/watchdog/watchdog.ko.xz"}},
		{"gl518sm", []string{"kernel/drivers/hwmon/gl518sm.ko.xz"}},
		{"msm", []string{"kernel/drivers/gpu/drm/msm/msm.ko",
			"kernel/drivers/gpu/drm/drm_kms_helper.ko"}},
	}
	for _, table := range tables {
		out, err := getModuleDeps(table.in, strings.NewReader(testModuleDep))
		if err != nil {
			t.Errorf("unexpected error with input: %q, error: %q", table.expected, err)
		}
		if !stringSlicesEqual(out, table.expected) {
			t.Errorf("Expected: %q, got: %q", table.expected, out)
		}
	}
}
