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
	"time"
	"github.com/zillode/notify"
)

type FsEvent struct {
	path string
}

type FsWatcher struct {
	folderPath string
	notifyModelChan chan<- []FsEvent
	FsEvents []FsEvent
	fsEventChan <-chan notify.EventInfo
	WatchingFs bool
	notifyDelay time.Duration
	notifyTimer time.Timer
	notifyTimerNeedsReset bool
}

func NewFsWatcher(folderPath string) *FsWatcher {
	return &FsWatcher{
		folderPath: folderPath,
		notifyModelChan: nil,
		FsEvents: make([]FsEvent, 0),
		fsEventChan: nil,
		WatchingFs: false,
		notifyDelay: fastNotifyDelay,
		notifyTimerNeedsReset: false,
	}
}

func ChangedSubfolders(events []FsEvent) []string {
	var paths []string
	for _, event := range events {
		paths = append(paths, event.path)
	}
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

func (watcher *FsWatcher) StartWatchingFilesystem() (<-chan []FsEvent, error) {
	notifyModelChan := make(chan []FsEvent)
	fsEventChan, err := setupNotifications(watcher.folderPath)
	if fsEventChan != nil {
		watcher.WatchingFs = true
		watcher.fsEventChan = fsEventChan
		go watcher.watchFilesystem()
	}
	watcher.notifyModelChan = notifyModelChan
	return notifyModelChan, err
}

func (watcher *FsWatcher) watchFilesystem() {
	watcher.notifyTimer = *time.NewTimer(watcher.notifyDelay)
	defer func() {
		watcher.notifyTimer.Stop()
	}()
	for {
		watcher.resetNotifyTimerIfNeeded()
		select {
		case event, _ := <- watcher.fsEventChan:
			watcher.speedUpNotifyTimer()
			newEvent := watcher.newFsEvent(event.Path())
			if newEvent != nil {
				watcher.FsEvents = append(watcher.FsEvents, *newEvent)
			}
		case <-watcher.notifyTimer.C:
			watcher.notifyTimerNeedsReset = true
			if len(watcher.FsEvents) > 0 {
				l.Debugf("Notifying about %d fs events\n", len(watcher.FsEvents))
				watcher.notifyModelChan <- watcher.FsEvents
			} else {
				watcher.slowDownNotifyTimer()
			}
			watcher.FsEvents = nil
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

func (watcher *FsWatcher) resetNotifyTimerIfNeeded() {
	if watcher.notifyTimerNeedsReset {
	l.Debugf("Resetting notifyTimer to %#v\n", watcher.notifyDelay)
		watcher.notifyTimer.Reset(watcher.notifyDelay)
		watcher.notifyTimerNeedsReset = false
	}
}

func (watcher *FsWatcher) speedUpNotifyTimer() {
	if watcher.notifyDelay != fastNotifyDelay {
		watcher.notifyDelay = fastNotifyDelay
		l.Debugf("Speeding up notifyTimer to %#v\n", watcher.notifyDelay)
		watcher.notifyTimerNeedsReset = true
	}
}

func (watcher *FsWatcher) slowDownNotifyTimer() {
	if watcher.notifyDelay != slowNotifyDelay {
		watcher.notifyDelay = slowNotifyDelay
		l.Debugf("Slowing down notifyTimer to %#v\n", watcher.notifyDelay)
		watcher.notifyTimerNeedsReset = true
	}
}

const (
	slowNotifyDelay = time.Duration(60) * time.Second
	fastNotifyDelay = time.Duration(500) * time.Millisecond
)
