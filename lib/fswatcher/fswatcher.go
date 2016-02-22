// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package fswatcher

import (
	"strings"
	"github.com/zillode/notify"
)

type fsEvent struct {
	path string
}

type FsWatcher struct {
	folderPath string
	EventsReady chan string
	fsEvents []fsEvent
	notifyChan chan notify.EventInfo
}

func NewFsWatcher(folderPath string, ) *FsWatcher {
	watcher := &FsWatcher{
		folderPath: folderPath,
		EventsReady: make(chan string),
		fsEvents: make([]fsEvent, 0),
	}
	return watcher
}

func (watcher *FsWatcher) ProcessEvents() {
	for _, event := range watcher.fsEvents {
		l.Debugf("Got event for: %#v", event)
	}
	watcher.fsEvents = nil
}

var maxFiles = 512

func setupNotifications(path string) chan notify.EventInfo {
	c := make(chan notify.EventInfo, maxFiles)
	if err := notify.Watch(path, c, notify.All); err != nil {
		l.Warnf("Failed to install inotify handler for %s. Error: %s",
			path, err)
		if strings.Contains(err.Error(), "too many open files") ||
			strings.Contains(err.Error(), "no space left on device") {
			l.Warnf("Please increase inotify limits, see http://bit.ly/1PxkdUC for more information.")
		}
		return nil
	}
	l.Debugf("Setup filesystem notification for %s", path)
	return c
}

func (watcher *FsWatcher) WaitForEvents() {
	watcher.notifyChan = setupNotifications(watcher.folderPath)
	defer notify.Stop(watcher.notifyChan)
	for {
		select {
		case event, _ := <- watcher.notifyChan:
			//l.Debugf("Got: %#v", event)
			watcher.fsEvents = append(watcher.fsEvents,
				fsEvent{event.Path()})
			watcher.EventsReady <- event.Path()
		}
	}
}
