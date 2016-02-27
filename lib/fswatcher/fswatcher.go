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
	"path/filepath"
	"strings"
	"github.com/zillode/notify"
)

type FsEvent struct {
	path string
}

type FsWatcher struct {
	folderPath string
	notifyModelChan chan<- bool
	FsEvents []FsEvent
	fsEventChan chan notify.EventInfo
}

func NewFsWatcher(notifyModelChan chan<- bool, folderPath string) (*FsWatcher, error) {
	fsEventChan, err := setupNotifications(folderPath)
	if err != nil {
		l.Warnln(err)
		return nil, err
	}
	watcher := &FsWatcher{
		folderPath: folderPath,
		notifyModelChan: notifyModelChan,
		FsEvents: make([]FsEvent, 0),
		fsEventChan: fsEventChan,
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
	defer notify.Stop(watcher.fsEventChan)
	for {
		select {
		case event, _ := <- watcher.fsEventChan:
			//l.Debugf("Got: %#v", event)
			// TODO: buffer events for a short time
			newEvent := watcher.newFsEvent(event.Path())
			if newEvent != nil {
				watcher.FsEvents = append(watcher.FsEvents, *newEvent)
			}
			if len(watcher.FsEvents) > 0 {
				watcher.notifyModelChan <- true
			}
		}
	}
}

func (watcher *FsWatcher) newFsEvent(eventPath string) *FsEvent {
	if isSubpath(eventPath, watcher.folderPath) {
		path := relativePath(eventPath, watcher.folderPath)
		return &FsEvent{path}
	}
	return nil
}

func relativePath(path string, folderPath string) string {
	subPath, _ := filepath.Rel(folderPath, path)
	return subPath
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
