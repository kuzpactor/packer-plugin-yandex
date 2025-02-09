// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package yandex

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const TestServiceAccountKeyFile = "./testdata/fake-sa-key.json"

func TestConfigPrepare(t *testing.T) {
	tf, err := ioutil.TempFile("", "packer")
	require.NoError(t, err, "create temporary file failed")

	defer os.Remove(tf.Name())
	tf.Close()

	cases := []struct {
		Key   string
		Value interface{}
		Err   bool
	}{
		{
			"unknown_key",
			"bad",
			true,
		},

		{
			"service_account_key_file",
			"/tmp/i/should/not/exist",
			true,
		},
		{
			"service_account_key_file",
			tf.Name(),
			true,
		},
		{
			"service_account_key_file",
			TestServiceAccountKeyFile,
			false,
		},

		{
			"folder_id",
			nil,
			true,
		},
		{
			"folder_id",
			"foo",
			false,
		},

		{
			"source_image_id",
			nil,
			true,
		},
		{
			"source_image_id",
			"foo",
			false,
		},

		{
			"source_image_family",
			nil,
			false,
		},
		{
			"source_image_family",
			"foo",
			false,
		},

		{
			"zone",
			nil,
			false,
		},
		{
			"zone",
			"foo",
			false,
		},

		{
			"ssh_timeout",
			"SO BAD",
			true,
		},
		{
			"ssh_timeout",
			"5s",
			false,
		},

		{
			"image_family",
			nil,
			false,
		},
		{
			"image_family",
			"",
			false,
		},
		{
			"image_family",
			"foo-bar",
			false,
		},
		{
			"image_family",
			"foo bar",
			true,
		},
	}

	for _, tc := range cases {
		raw := testConfig(t)

		if tc.Value == nil {
			delete(raw, tc.Key)
		} else {
			raw[tc.Key] = tc.Value
		}

		if tc.Key == "service_account_key_file" {
			delete(raw, "token")
		}

		var c Config
		warns, errs := c.Prepare(raw)

		if tc.Err {
			testConfigErr(t, warns, errs, tc.Key)
		} else {
			testConfigOk(t, warns, errs)
		}
	}
}

func TestConfigPrepareStartupScriptFile(t *testing.T) {
	config := testConfig(t)

	config["metadata"] = map[string]string{
		"key": "value",
	}

	config["metadata_from_file"] = map[string]string{
		"key": "file_not_exist",
	}

	var c Config
	_, errs := c.Prepare(config)

	if errs == nil || !strings.Contains(errs.Error(), "cannot access file 'file_not_exist' with content "+
		"for value of metadata key 'key':") {
		t.Fatalf("should error: metadata_from_file")
	}
}

func TestConfigImageMinDiskSize(t *testing.T) {
	cases := []struct {
		Name   string
		Config map[string]interface{}
		Err    bool
	}{
		{
			Name: "image_min_disk_size lower than disk_size (default value)",
			Config: map[string]interface{}{
				"image_min_disk_size_gb": 2,
			},
			Err: true,
		},
		{
			Name: "image_min_disk_size greater than disk_size (default value)",
			Config: map[string]interface{}{
				"image_min_disk_size_gb": 20,
			},
			Err: false,
		},
		{
			Name: "image_min_disk_size lower than disk_size (custom value)",
			Config: map[string]interface{}{
				"disk_size_gb":           50,
				"image_min_disk_size_gb": 20,
			},
			Err: true,
		},
		{
			Name: "image_min_disk_size greate than disk_size (custom value)",
			Config: map[string]interface{}{
				"disk_size_gb":           50,
				"image_min_disk_size_gb": 55,
			},
			Err: false,
		},
	}

	for _, tc := range cases {
		raw := testConfig(t)

		for k, v := range tc.Config {
			raw[k] = v
		}

		var c Config
		warns, errs := c.Prepare(raw)

		if tc.Err {
			testConfigErr(t, warns, errs, tc.Name)
		} else {
			testConfigOk(t, warns, errs)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	cases := []struct {
		Read  func(c *Config) interface{}
		Value interface{}
	}{
		{
			func(c *Config) interface{} { return c.Communicator.Type },
			"ssh",
		},

		{
			func(c *Config) interface{} { return c.Communicator.SSHPort },
			22,
		},

		{
			func(c *Config) interface{} { return c.TargetImageFolderID },
			"hashicorp",
		},
	}

	for _, tc := range cases {
		raw := testConfig(t)

		var c Config
		warns, errs := c.Prepare(raw)
		testConfigOk(t, warns, errs)

		actual := tc.Read(&c)
		if actual != tc.Value {
			t.Fatalf("bad: %#v", actual)
		}
	}
}

func TestImageName(t *testing.T) {
	raw := testConfig(t)

	var c Config

	warns, errs := c.Prepare(raw)
	testConfigOk(t, warns, errs)

	if !strings.HasPrefix(c.ImageName, "packer-") {
		t.Fatalf("ImageName should have 'packer-' prefix, found %s", c.ImageName)
	}
	if strings.Contains(c.ImageName, "{{timestamp}}") {
		t.Errorf("ImageName should be interpolated; found %s", c.ImageName)
	}
}

func TestZone(t *testing.T) {
	raw := testConfig(t)

	var c Config
	warns, errs := c.Prepare(raw)
	testConfigOk(t, warns, errs)
	if c.Zone != "ru-central1-a" {
		t.Fatalf("Zone should be 'ru-central1-a' given, but is '%s'", c.Zone)
	}
}

func TestGpuDefaultPlatformID(t *testing.T) {
	raw := testConfig(t)
	raw["instance_gpus"] = 1

	var c Config
	warns, errs := c.Prepare(raw)
	testConfigOk(t, warns, errs)
	if c.PlatformID != "gpu-standard-v1" {
		t.Fatalf("expected 'gpu-standard-v1' as default platform_id for instance with GPU(s), but got '%s'", c.PlatformID)
	}
}

// Helper stuff below

func testConfig(t *testing.T) (config map[string]interface{}) {
	config = map[string]interface{}{
		"token":           "test_token",
		"folder_id":       "hashicorp",
		"source_image_id": "foo",
		"ssh_username":    "root",
		"image_family":    "bar",
		"image_product_ids": []string{
			"test-license",
		},
		"zone": "ru-central1-a",
	}

	return config
}

func testConfigStruct(t *testing.T) *Config {
	raw := testConfig(t)

	var c Config
	warns, errs := c.Prepare(raw)

	require.True(t, len(warns) == 0, "bad: %#v", warns)
	require.NoError(t, errs, "should not have error: %s", errs)

	return &c
}

func testConfigErr(t *testing.T, warns []string, err error, extra string) {
	if len(warns) > 0 {
		t.Fatalf("bad: %#v", warns)
	}
	if err == nil {
		t.Fatalf("should error: %s", extra)
	}
}

func testConfigOk(t *testing.T, warns []string, err error) {
	if len(warns) > 0 {
		t.Fatalf("bad: %#v", warns)
	}
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
}
