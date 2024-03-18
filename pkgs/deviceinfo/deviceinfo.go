// Copyright 2021 Clayton Craft <clayton@craftyguy.net>
// SPDX-License-Identifier: GPL-3.0-or-later

package deviceinfo

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strings"

	"gitlab.com/postmarketOS/postmarketos-mkinitfs/internal/misc"
)

type DeviceInfo struct {
	InitfsCompression      string
	InitfsExtraCompression string
	UbootBoardname         string
	GenerateSystemdBoot    string
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

	fd, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fd.Close()

	if err := d.unmarshal(fd); err != nil {
		return err
	}

	return nil
}

// Unmarshals a deviceinfo into a DeviceInfo struct
func (d *DeviceInfo) unmarshal(r io.Reader) error {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		// line isn't setting anything, so just ignore it
		if !strings.Contains(line, "=") {
			continue
		}

		// sometimes line has a comment at the end after setting an option
		line = strings.SplitN(line, "#", 2)[0]
		line = strings.TrimSpace(line)

		// must support having '=' in the value (e.g. kernel cmdline)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("error parsing deviceinfo line, invalid format: %s", line)
		}

		name, val := parts[0], parts[1]
		val = strings.ReplaceAll(val, "\"", "")

		if name == "deviceinfo_format_version" && val != "0" {
			return fmt.Errorf("deviceinfo format version %q is not supported", val)
		}

		fieldName := nameToField(name)

		if fieldName == "" {
			return fmt.Errorf("error parsing deviceinfo line, invalid format: %s", line)
		}

		field := reflect.ValueOf(d).Elem().FieldByName(fieldName)
		if !field.IsValid() {
			// an option that meets the deviceinfo "specification", but isn't
			// one we care about in this module
			continue
		}
		field.SetString(val)
	}
	if err := s.Err(); err != nil {
		log.Print("unable to parse deviceinfo: ", err)
		return err
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
	}`,
		"deviceinfo_format_version", d.FormatVersion,
		"deviceinfo_initfs_compression", d.InitfsCompression,
		"deviceinfo_initfs_extra_compression", d.InitfsCompression,
		"deviceinfo_ubootBoardname", d.UbootBoardname,
		"deviceinfo_generateSystemdBoot", d.GenerateSystemdBoot,
		"deviceinfo_formatVersion", d.FormatVersion,
	)
}
