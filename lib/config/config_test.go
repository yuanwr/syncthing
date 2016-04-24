// Copyright (C) 2014 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/d4l3k/messagediff"
	"github.com/syncthing/syncthing/lib/protocol"
)

var device1, device2, device3, device4 protocol.DeviceID

func init() {
	device1, _ = protocol.DeviceIDFromString("AIR6LPZ7K4PTTUXQSMUUCPQ5YWOEDFIIQJUG7772YQXXR5YD6AWQ")
	device2, _ = protocol.DeviceIDFromString("GYRZZQB-IRNPV4Z-T7TC52W-EQYJ3TT-FDQW6MW-DFLMU42-SSSU6EM-FBK2VAY")
	device3, _ = protocol.DeviceIDFromString("LGFPDIT-7SKNNJL-VJZA4FC-7QNCRKA-CE753K7-2BW5QDK-2FOZ7FR-FEP57QJ")
	device4, _ = protocol.DeviceIDFromString("P56IOI7-MZJNU2Y-IQGDREY-DM2MGTI-MGL3BXN-PQ6W5BM-TBBZ4TJ-XZWICQ2")
}

func TestDefaultValues(t *testing.T) {
	expected := OptionsConfiguration{
		ListenAddresses:         []string{"default"},
		GlobalAnnServers:        []string{"default"},
		GlobalAnnEnabled:        true,
		LocalAnnEnabled:         true,
		LocalAnnPort:            21027,
		LocalAnnMCAddr:          "[ff12::8384]:21027",
		MaxSendKbps:             0,
		MaxRecvKbps:             0,
		ReconnectIntervalS:      60,
		RelayReconnectIntervalM: 10,
		StartBrowser:            true,
		NATEnabled:              true,
		NATLeaseM:               60,
		NATRenewalM:             30,
		NATTimeoutS:             10,
		RestartOnWakeup:         true,
		AutoUpgradeIntervalH:    12,
		KeepTemporariesH:        24,
		CacheIgnoredFiles:       false,
		ProgressUpdateIntervalS: 5,
		SymlinksEnabled:         true,
		LimitBandwidthInLan:     false,
		MinHomeDiskFreePct:      1,
		URURL:                   "https://data.syncthing.net/newdata",
		URInitialDelayS:         1800,
		URPostInsecurely:        false,
		ReleasesURL:             "https://api.github.com/repos/syncthing/syncthing/releases?per_page=30",
		AlwaysLocalNets:         []string{},
		OverwriteNames:          false,
		TempIndexMinBlocks:      10,
		StunServers:             []string{"default"},
		StunRenewalM:            30,
	}

	cfg := New(device1)

	if diff, equal := messagediff.PrettyDiff(expected, cfg.Options); !equal {
		t.Errorf("Default config differs. Diff:\n%s", diff)
	}
}

func TestDeviceConfig(t *testing.T) {
	for i := OldestHandledVersion; i <= CurrentVersion; i++ {
		os.Remove("testdata/.stfolder")
		wr, err := Load(fmt.Sprintf("testdata/v%d.xml", i), device1)
		if err != nil {
			t.Fatal(err)
		}

		_, err = os.Stat("testdata/.stfolder")
		if i < 6 && err != nil {
			t.Fatal(err)
		} else if i >= 6 && err == nil {
			t.Fatal("Unexpected file")
		}

		cfg := wr.cfg

		expectedFolders := []FolderConfiguration{
			{
				ID:              "test",
				RawPath:         "testdata",
				Devices:         []FolderDeviceConfiguration{{DeviceID: device1}, {DeviceID: device4}},
				ReadOnly:        true,
				RescanIntervalS: 600,
				Copiers:         0,
				Pullers:         0,
				Hashers:         0,
				AutoNormalize:   true,
				MinDiskFreePct:  1,
				MaxConflicts:    -1,
			},
		}

		// The cachedPath will have been resolved to an absolute path,
		// depending on where the tests are running. Zero it out so we don't
		// fail based on that.
		for i := range cfg.Folders {
			cfg.Folders[i].cachedPath = ""
		}

		if runtime.GOOS != "windows" {
			expectedFolders[0].RawPath += string(filepath.Separator)
		}

		expectedDevices := []DeviceConfiguration{
			{
				DeviceID:    device1,
				Name:        "node one",
				Addresses:   []string{"tcp://a"},
				Compression: protocol.CompressMetadata,
			},
			{
				DeviceID:    device4,
				Name:        "node two",
				Addresses:   []string{"tcp://b"},
				Compression: protocol.CompressMetadata,
			},
		}
		expectedDeviceIDs := []protocol.DeviceID{device1, device4}

		if cfg.Version != CurrentVersion {
			t.Errorf("%d: Incorrect version %d != %d", i, cfg.Version, CurrentVersion)
		}
		if diff, equal := messagediff.PrettyDiff(expectedFolders, cfg.Folders); !equal {
			t.Errorf("%d: Incorrect Folders. Diff:\n%s", i, diff)
		}
		if diff, equal := messagediff.PrettyDiff(expectedDevices, cfg.Devices); !equal {
			t.Errorf("%d: Incorrect Devices. Diff:\n%s", i, diff)
		}
		if diff, equal := messagediff.PrettyDiff(expectedDeviceIDs, cfg.Folders[0].DeviceIDs()); !equal {
			t.Errorf("%d: Incorrect DeviceIDs. Diff:\n%s", i, diff)
		}
	}
}

func TestNoListenAddresses(t *testing.T) {
	cfg, err := Load("testdata/noListenAddress.xml", device1)
	if err != nil {
		t.Error(err)
	}

	expected := []string{""}
	actual := cfg.Options().ListenAddresses
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Unexpected ListenAddresses. Diff:\n%s", diff)
	}
}

func TestOverriddenValues(t *testing.T) {
	expected := OptionsConfiguration{
		ListenAddresses:         []string{"tcp://:23000"},
		GlobalAnnServers:        []string{"udp4://syncthing.nym.se:22026"},
		GlobalAnnEnabled:        false,
		LocalAnnEnabled:         false,
		LocalAnnPort:            42123,
		LocalAnnMCAddr:          "quux:3232",
		MaxSendKbps:             1234,
		MaxRecvKbps:             2341,
		ReconnectIntervalS:      6000,
		RelayReconnectIntervalM: 20,
		StartBrowser:            false,
		NATEnabled:              false,
		NATLeaseM:               90,
		NATRenewalM:             15,
		NATTimeoutS:             15,
		RestartOnWakeup:         false,
		AutoUpgradeIntervalH:    24,
		KeepTemporariesH:        48,
		CacheIgnoredFiles:       true,
		ProgressUpdateIntervalS: 10,
		SymlinksEnabled:         false,
		LimitBandwidthInLan:     true,
		MinHomeDiskFreePct:      5.2,
		URURL:                   "https://localhost/newdata",
		URInitialDelayS:         800,
		URPostInsecurely:        true,
		ReleasesURL:             "https://localhost/releases",
		AlwaysLocalNets:         []string{},
		OverwriteNames:          true,
		TempIndexMinBlocks:      100,
		StunServers:             []string{"stunning.com"},
		StunRenewalM:            10,
	}

	cfg, err := Load("testdata/overridenvalues.xml", device1)
	if err != nil {
		t.Error(err)
	}

	if diff, equal := messagediff.PrettyDiff(expected, cfg.Options()); !equal {
		t.Errorf("Overridden config differs. Diff:\n%s", diff)
	}
}

func TestDeviceAddressesDynamic(t *testing.T) {
	name, _ := os.Hostname()
	expected := map[protocol.DeviceID]DeviceConfiguration{
		device1: {
			DeviceID:  device1,
			Addresses: []string{"dynamic"},
		},
		device2: {
			DeviceID:  device2,
			Addresses: []string{"dynamic"},
		},
		device3: {
			DeviceID:  device3,
			Addresses: []string{"dynamic"},
		},
		device4: {
			DeviceID:    device4,
			Name:        name, // Set when auto created
			Addresses:   []string{"dynamic"},
			Compression: protocol.CompressMetadata,
		},
	}

	cfg, err := Load("testdata/deviceaddressesdynamic.xml", device4)
	if err != nil {
		t.Error(err)
	}

	actual := cfg.Devices()
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Devices differ. Diff:\n%s", diff)
	}
}

func TestDeviceCompression(t *testing.T) {
	name, _ := os.Hostname()
	expected := map[protocol.DeviceID]DeviceConfiguration{
		device1: {
			DeviceID:    device1,
			Addresses:   []string{"dynamic"},
			Compression: protocol.CompressMetadata,
		},
		device2: {
			DeviceID:    device2,
			Addresses:   []string{"dynamic"},
			Compression: protocol.CompressMetadata,
		},
		device3: {
			DeviceID:    device3,
			Addresses:   []string{"dynamic"},
			Compression: protocol.CompressNever,
		},
		device4: {
			DeviceID:    device4,
			Name:        name, // Set when auto created
			Addresses:   []string{"dynamic"},
			Compression: protocol.CompressMetadata,
		},
	}

	cfg, err := Load("testdata/devicecompression.xml", device4)
	if err != nil {
		t.Error(err)
	}

	actual := cfg.Devices()
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Devices differ. Diff:\n%s", diff)
	}
}

func TestDeviceAddressesStatic(t *testing.T) {
	name, _ := os.Hostname()
	expected := map[protocol.DeviceID]DeviceConfiguration{
		device1: {
			DeviceID:  device1,
			Addresses: []string{"tcp://192.0.2.1", "tcp://192.0.2.2"},
		},
		device2: {
			DeviceID:  device2,
			Addresses: []string{"tcp://192.0.2.3:6070", "tcp://[2001:db8::42]:4242"},
		},
		device3: {
			DeviceID:  device3,
			Addresses: []string{"tcp://[2001:db8::44]:4444", "tcp://192.0.2.4:6090"},
		},
		device4: {
			DeviceID:    device4,
			Name:        name, // Set when auto created
			Addresses:   []string{"dynamic"},
			Compression: protocol.CompressMetadata,
		},
	}

	cfg, err := Load("testdata/deviceaddressesstatic.xml", device4)
	if err != nil {
		t.Error(err)
	}

	actual := cfg.Devices()
	if diff, equal := messagediff.PrettyDiff(expected, actual); !equal {
		t.Errorf("Devices differ. Diff:\n%s", diff)
	}
}

func TestVersioningConfig(t *testing.T) {
	cfg, err := Load("testdata/versioningconfig.xml", device4)
	if err != nil {
		t.Error(err)
	}

	vc := cfg.Folders()["test"].Versioning
	if vc.Type != "simple" {
		t.Errorf(`vc.Type %q != "simple"`, vc.Type)
	}
	if l := len(vc.Params); l != 2 {
		t.Errorf("len(vc.Params) %d != 2", l)
	}

	expected := map[string]string{
		"foo": "bar",
		"baz": "quux",
	}
	if diff, equal := messagediff.PrettyDiff(expected, vc.Params); !equal {
		t.Errorf("vc.Params differ. Diff:\n%s", diff)
	}
}

func TestIssue1262(t *testing.T) {
	cfg, err := Load("testdata/issue-1262.xml", device4)
	if err != nil {
		t.Fatal(err)
	}

	actual := cfg.Folders()["test"].RawPath
	expected := "e:/"
	if runtime.GOOS == "windows" {
		expected = `e:\`
	}

	if actual != expected {
		t.Errorf("%q != %q", actual, expected)
	}
}

func TestIssue1750(t *testing.T) {
	cfg, err := Load("testdata/issue-1750.xml", device4)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Options().ListenAddresses[0] != "tcp://:23000" {
		t.Errorf("%q != %q", cfg.Options().ListenAddresses[0], "tcp://:23000")
	}

	if cfg.Options().ListenAddresses[1] != "tcp://:23001" {
		t.Errorf("%q != %q", cfg.Options().ListenAddresses[1], "tcp://:23001")
	}

	if cfg.Options().GlobalAnnServers[0] != "udp4://syncthing.nym.se:22026" {
		t.Errorf("%q != %q", cfg.Options().GlobalAnnServers[0], "udp4://syncthing.nym.se:22026")
	}

	if cfg.Options().GlobalAnnServers[1] != "udp4://syncthing.nym.se:22027" {
		t.Errorf("%q != %q", cfg.Options().GlobalAnnServers[1], "udp4://syncthing.nym.se:22027")
	}
}

func TestWindowsPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Not useful on non-Windows")
		return
	}

	folder := FolderConfiguration{
		RawPath: `e:\`,
	}

	expected := `\\?\e:\`
	actual := folder.Path()
	if actual != expected {
		t.Errorf("%q != %q", actual, expected)
	}

	folder.RawPath = `\\192.0.2.22\network\share`
	expected = folder.RawPath
	actual = folder.Path()
	if actual != expected {
		t.Errorf("%q != %q", actual, expected)
	}

	folder.RawPath = `relative\path`
	expected = folder.RawPath
	actual = folder.Path()
	if actual == expected || !strings.HasPrefix(actual, "\\\\?\\") {
		t.Errorf("%q == %q, expected absolutification", actual, expected)
	}
}

func TestFolderPath(t *testing.T) {
	folder := FolderConfiguration{
		RawPath: "~/tmp",
	}

	realPath := folder.Path()
	if !filepath.IsAbs(realPath) {
		t.Error(realPath, "should be absolute")
	}
	if strings.Contains(realPath, "~") {
		t.Error(realPath, "should not contain ~")
	}
}

func TestNewSaveLoad(t *testing.T) {
	path := "testdata/temp.xml"
	os.Remove(path)

	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	intCfg := New(device1)
	cfg := Wrap(path, intCfg)

	// To make the equality pass later
	cfg.cfg.XMLName.Local = "configuration"

	if exists(path) {
		t.Error(path, "exists")
	}

	err := cfg.Save()
	if err != nil {
		t.Error(err)
	}
	if !exists(path) {
		t.Error(path, "does not exist")
	}

	cfg2, err := Load(path, device1)
	if err != nil {
		t.Error(err)
	}

	if diff, equal := messagediff.PrettyDiff(cfg.Raw(), cfg2.Raw()); !equal {
		t.Errorf("Configs are not equal. Diff:\n%s", diff)
	}

	os.Remove(path)
}

func TestPrepare(t *testing.T) {
	var cfg Configuration

	if cfg.Folders != nil || cfg.Devices != nil || cfg.Options.ListenAddresses != nil {
		t.Error("Expected nil")
	}

	cfg.prepare(device1)

	if cfg.Folders == nil || cfg.Devices == nil || cfg.Options.ListenAddresses == nil {
		t.Error("Unexpected nil")
	}
}

func TestCopy(t *testing.T) {
	wrapper, err := Load("testdata/example.xml", device1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := wrapper.Raw()

	bsOrig, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	copy := cfg.Copy()

	cfg.Devices[0].Addresses[0] = "wrong"
	cfg.Folders[0].Devices[0].DeviceID = protocol.DeviceID{0, 1, 2, 3}
	cfg.Options.ListenAddresses[0] = "wrong"
	cfg.GUI.APIKey = "wrong"

	bsChanged, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	bsCopy, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(bsOrig, bsChanged) {
		t.Error("Config should have changed")
	}
	if !bytes.Equal(bsOrig, bsCopy) {
		//ioutil.WriteFile("a", bsOrig, 0644)
		//ioutil.WriteFile("b", bsCopy, 0644)
		t.Error("Copy should be unchanged")
	}
}

func TestPullOrder(t *testing.T) {
	wrapper, err := Load("testdata/pullorder.xml", device1)
	if err != nil {
		t.Fatal(err)
	}
	folders := wrapper.Folders()

	expected := []struct {
		name  string
		order PullOrder
	}{
		{"f1", OrderRandom},        // empty value, default
		{"f2", OrderRandom},        // explicit
		{"f3", OrderAlphabetic},    // explicit
		{"f4", OrderRandom},        // unknown value, default
		{"f5", OrderSmallestFirst}, // explicit
		{"f6", OrderLargestFirst},  // explicit
		{"f7", OrderOldestFirst},   // explicit
		{"f8", OrderNewestFirst},   // explicit
	}

	// Verify values are deserialized correctly

	for _, tc := range expected {
		if actual := folders[tc.name].Order; actual != tc.order {
			t.Errorf("Incorrect pull order for %q: %v != %v", tc.name, actual, tc.order)
		}
	}

	// Serialize and deserialize again to verify it survives the transformation

	buf := new(bytes.Buffer)
	cfg := wrapper.Raw()
	cfg.WriteXML(buf)

	t.Logf("%s", buf.Bytes())

	cfg, err = ReadXML(buf, device1)
	wrapper = Wrap("testdata/pullorder.xml", cfg)
	folders = wrapper.Folders()

	for _, tc := range expected {
		if actual := folders[tc.name].Order; actual != tc.order {
			t.Errorf("Incorrect pull order for %q: %v != %v", tc.name, actual, tc.order)
		}
	}
}

func TestLargeRescanInterval(t *testing.T) {
	wrapper, err := Load("testdata/largeinterval.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	if wrapper.Folders()["l1"].RescanIntervalS != MaxRescanIntervalS {
		t.Error("too large rescan interval should be maxed out")
	}
	if wrapper.Folders()["l2"].RescanIntervalS != 0 {
		t.Error("negative rescan interval should become zero")
	}
}

func TestGUIConfigURL(t *testing.T) {
	testcases := [][2]string{
		{"192.0.2.42:8080", "http://192.0.2.42:8080/"},
		{":8080", "http://127.0.0.1:8080/"},
		{"0.0.0.0:8080", "http://127.0.0.1:8080/"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080/"},
		{"127.0.0.2:8080", "http://127.0.0.2:8080/"},
		{"[::]:8080", "http://[::1]:8080/"},
		{"[2001::42]:8080", "http://[2001::42]:8080/"},
	}

	for _, tc := range testcases {
		c := GUIConfiguration{
			RawAddress: tc[0],
		}
		u := c.URL()
		if u != tc[1] {
			t.Errorf("Incorrect URL %s != %s for addr %s", u, tc[1], tc[0])
		}
	}
}

func TestRemoveDuplicateDevicesFolders(t *testing.T) {
	wrapper, err := Load("testdata/duplicates.xml", device1)
	if err != nil {
		t.Fatal(err)
	}

	// All folders are loaded, but the duplicate ones are disabled.
	if l := len(wrapper.Raw().Folders); l != 3 {
		t.Errorf("Incorrect number of folders, %d != 3", l)
	}
	for i, f := range wrapper.Raw().Folders {
		if f.ID == "f1" && f.Invalid == "" {
			t.Errorf("Folder %d (%q) is not set invalid", i, f.ID)
		}
	}

	if l := len(wrapper.Raw().Devices); l != 3 {
		t.Errorf("Incorrect number of devices, %d != 3", l)
	}

	f := wrapper.Folders()["f2"]
	if l := len(f.Devices); l != 2 {
		t.Errorf("Incorrect number of folder devices, %d != 2", l)
	}
}
