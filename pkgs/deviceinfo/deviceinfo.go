// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package deviceinfo

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mvdan/sh/shell"
	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
)

type DeviceInfo struct {
	InitfsCompression      string
	InitfsExtraCompression string
	UbootBoardname         string
	GenerateSystemdBoot    string
	FormatVersion          string
	CreateInitfsExtra      bool
}

// Reads the relevant entries from "file" into DeviceInfo struct
// Any already-set entries will be overwriten if they are present
// in "file"
func (d *DeviceInfo) ReadDeviceinfo(file string) error {
	if exists, err := misc.Exists(file); !exists {
		return fmt.Errorf("%q not found, required by mkinitfs", file)
	} else if err != nil {
		return fmt.Errorf("unexpected error getting status for %q: %s", file, err)
	}

	if err := d.unmarshal(file); err != nil {
		return err
	}

	return nil
}

// Unmarshals a deviceinfo into a DeviceInfo struct
func (d *DeviceInfo) unmarshal(file string) error {
	ctx, cancelCtx := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancelCtx()
	vars, err := shell.SourceFile(ctx, file)
	if err != nil {
		return fmt.Errorf("parsing deviceinfo %q failed: %w", file, err)
	}

	for k, v := range vars {
		fieldName := nameToField(k)
		field := reflect.ValueOf(d).Elem().FieldByName(fieldName)
		if !field.IsValid() {
			// an option that meets the deviceinfo "specification", but isn't
			// one we care about in this module
			continue
		}
		switch field.Interface().(type) {
		case string:
			field.SetString(v.String())
		case bool:
			if v, err := strconv.ParseBool(v.String()); err != nil {
				return fmt.Errorf("deviceinfo %q has unsupported type for field %q, expected 'bool'", file, k)
			} else {
				field.SetBool(v)
			}
		case int:
			if v, err := strconv.ParseInt(v.String(), 10, 32); err != nil {
				return fmt.Errorf("deviceinfo %q has unsupported type for field %q, expected 'int'", file, k)
			} else {
				field.SetInt(v)
			}
		default:
			return fmt.Errorf("deviceinfo %q has unsupported type for field %q", file, k)
		}
	}

	if d.FormatVersion != "0" {
		return fmt.Errorf("deviceinfo %q has an unsupported format version %q", file, d.FormatVersion)
	}

	return nil
}

// Convert string into the string format used for DeviceInfo fields.
// Note: does not test that the resulting field name is a valid field in the
// DeviceInfo struct!
func nameToField(name string) string {
	var field string
	parts := strings.Split(name, "_")
	for _, p := range parts {
		if p == "deviceinfo" {
			continue
		}
		if len(p) < 1 {
			continue
		}
		field = field + strings.ToUpper(p[:1]) + p[1:]
	}

	return field
}

func (d DeviceInfo) String() string {
	return fmt.Sprintf(`{
			%s: %v
			%s: %v
			%s: %v
			%s: %v
			%s: %v
			%s: %v
			%s: %v
			%s: %v
	}`,
		"deviceinfo_format_version", d.FormatVersion,
		"deviceinfo_", d.FormatVersion,
		"deviceinfo_initfs_compression", d.InitfsCompression,
		"deviceinfo_initfs_extra_compression", d.InitfsCompression,
		"deviceinfo_ubootBoardname", d.UbootBoardname,
		"deviceinfo_generateSystemdBoot", d.GenerateSystemdBoot,
		"deviceinfo_formatVersion", d.FormatVersion,
		"deviceinfo_createInitfsExtra", d.CreateInitfsExtra,
	)
}
