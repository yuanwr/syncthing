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

	"github.com/syncthing/syncthing/lib/events"
)

type FsEvent struct {
	path string
}

type FsWatcher struct {
	folderPath string
	notifyModelChan chan<- map[string]FsEvent
	fsEvents map[string]FsEvent
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
		fsEvents: make(map[string]FsEvent),
		fsEventChan: nil,
		WatchingFs: false,
		notifyDelay: fastNotifyDelay,
		notifyTimerNeedsReset: false,
	}
}

func (watcher *FsWatcher) StartWatchingFilesystem() (<-chan map[string]FsEvent, error) {
	notifyModelChan := make(chan map[string]FsEvent)
	fsEventChan, err := setupNotifications(watcher.folderPath)
	if fsEventChan != nil {
		watcher.WatchingFs = true
		watcher.fsEventChan = fsEventChan
		go watcher.watchFilesystem()
	}
	watcher.notifyModelChan = notifyModelChan
	return notifyModelChan, err
}

func ExtractChangedPaths(events map[string]FsEvent) []string {
	var paths []string
	for path := range events {
		paths = append(paths, path)
	}
	return paths
}

var maxFiles = 512

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

func (watcher *FsWatcher) watchFilesystem() {
	watcher.notifyTimer = *time.NewTimer(watcher.notifyDelay)
	defer watcher.notifyTimer.Stop()
	finishedFileEventSubscription := events.Default.Subscribe(events.ItemFinished)
	for {
		watcher.resetNotifyTimerIfNeeded()
		select {
		case event, _ := <- watcher.fsEventChan:
			watcher.speedUpNotifyTimer()
			watcher.storeFsEvent(event)
		case <-watcher.notifyTimer.C:
			watcher.sendStoredEventsToModelOrSlowDownTimer()
		case event := <-finishedFileEventSubscription.C():
			watcher.skipPathChangedByUs(event)
		}
	}
}

func (watcher *FsWatcher) newFsEvent(eventPath string) *FsEvent {
	if isSubpath(eventPath, watcher.folderPath) {
		path := relativePath(eventPath, watcher.folderPath)
		if !isSpecialPath(path) {
			return &FsEvent{path}
		}
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

func (watcher *FsWatcher) addEvent(event FsEvent) {
	watcher.fsEvents[event.path] = event
}

func (watcher *FsWatcher) removeEventIfPresent(path string) {
	delete(watcher.fsEvents, path)
}

func (watcher *FsWatcher) storeFsEvent(event notify.EventInfo) {
	newEvent := watcher.newFsEvent(event.Path())
	if newEvent != nil {
		watcher.addEvent(*newEvent)
	}
}
func (watcher *FsWatcher) sendStoredEventsToModelOrSlowDownTimer() {
	watcher.notifyTimerNeedsReset = true
	if len(watcher.fsEvents) > 0 {
		l.Debugf("Notifying about %d fs events\n", len(watcher.fsEvents))
		watcher.notifyModelChan <- watcher.fsEvents
	} else {
		watcher.slowDownNotifyTimer()
	}
	watcher.fsEvents = make(map[string]FsEvent)
}

func (watcher *FsWatcher) skipPathChangedByUs(event events.Event) {
	path := event.Data.(map[string]interface{})["item"].(string)
	l.Debugf("Skipping notification for finished path: %s\n", path)
	watcher.removeEventIfPresent(path)
}

func isSpecialPath(path string) bool {
	return strings.Contains(path, ".syncthing.") && strings.HasSuffix(path, ".tmp")
}
