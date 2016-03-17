// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration

package integration

import (
	"fmt"
	"log"
	"os"
	"testing"
)

func TestIgnoreOurOwnFsEvents(t *testing.T) {
	log.Println("Starting sender instance...")
	cleanUp(t, 1)
	sender := startInstance(t, 1)
	defer checkedStop(t, sender)
	log.Println("Starting receiver instance...")
	cleanUp(t, 2)
	receiver := startInstance(t, 2)
	defer checkedStop(t, receiver)

	dirs := []string{"d1"}
	files := []string{"d1/f1.TXT"}
	all := append(files, dirs...)

	log.Println("Creating directories and files...")
	createDirectories(t, "s1", dirs)
	createFiles(t, "s1", files)
	rescan(t, sender)
	log.Println("Waiting for sender to sync...")
	waitForSync(t, sender)

	model := getModel(t, sender, "default")
	expected := len(all)
	if model.LocalFiles != expected {
		t.Fatalf("Incorrect number of files after initial scan, %d != %d", model.LocalFiles, expected)
	}
}

func cleanUp(t *testing.T, index int) {
	log.Println("Cleaning up test files/directories...")
	err := removeAll(fmt.Sprintf("s%d", index),
		fmt.Sprintf("h%d/index*", index))
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(fmt.Sprintf("s%d", index), 0755)
	if err != nil {
		t.Fatal(err)
	}
}
