// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package fswatcher

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"github.com/zillode/notify"
)

type FsEvent struct {
	path string
}

type FsWatcher struct {
	folderPath string
	folderModelChan  chan FsEvent
	FsEvents []FsEvent
	notifyChan chan notify.EventInfo
}

func NewFsWatcher(folderModelChan chan FsEvent, folderPath string) (*FsWatcher, error) {
	notifyChan, err := setupNotifications(folderPath)
	if err != nil {
		l.Warnln(err)
		return nil, err
	}
	watcher := &FsWatcher{
		folderPath: folderPath,
		folderModelChan: folderModelChan,
		FsEvents: make([]FsEvent, 0),
		notifyChan: notifyChan,
	}
	return watcher, nil
}

func (watcher *FsWatcher) ChangedSubfolders() []string {
	var paths []string
	for _, event := range watcher.FsEvents {
		l.Debugf("Got event for: %#v", event)
		paths = append(paths, event.path)
	}
	watcher.FsEvents = nil
	return paths
}

var maxFiles = 512

type fsWatcherError struct {
	originalError error
}

func setupNotifications(path string) (chan notify.EventInfo, error) {
	c := make(chan notify.EventInfo, maxFiles)
	if err := notify.Watch(path, c, notify.All); err != nil {
		if strings.Contains(err.Error(), "too many open files") ||
			strings.Contains(err.Error(), "no space left on device") {
			return nil, errors.New("Please increase inotify limits, see http://bit.ly/1PxkdUC for more information.")
		}
		return nil, fmt.Errorf(
			"Failed to install inotify handler for %s. Error: %s",
			path, err)
	}
	l.Debugf("Setup filesystem notification for %s", path)
	return c, nil
}

func (watcher *FsWatcher) WaitForEvents() {
	defer notify.Stop(watcher.notifyChan)
	for {
		select {
		case event, _ := <- watcher.notifyChan:
			//l.Debugf("Got: %#v", event)
			// TODO: get rid of this FsEvents list
			watcher.FsEvents = append(watcher.FsEvents,
				FsEvent{event.Path()})
			watcher.folderModelChan <- FsEvent{event.Path()}
		}
	}
}

func relativePath(path string, folderPath string) string {
	if len(folderPath) > 1 &&
		os.IsPathSeparator(folderPath[len(folderPath) - 1]) {
		folderPath = folderPath[0:len(folderPath)-1]
	}
	if strings.HasPrefix(path, folderPath) {
		path = strings.TrimPrefix(path, folderPath)
		if len(path) != 0 && os.IsPathSeparator(path[0]) {
			path = path[1:len(path)]
		}
	}
	return path
}

func isSubpath(path string, folderPath string) bool {
	if len(path) > 1 && os.IsPathSeparator(path[len(path) - 1]) {
		path = path[0:len(path)-1]
	}
	if len(folderPath) > 1 && os.IsPathSeparator(folderPath[len(folderPath) - 1]) {
		folderPath = folderPath[0:len(folderPath)-1]
	}
	return strings.HasPrefix(path, folderPath)
}
