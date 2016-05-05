// Copyright (C) 2014 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package model

import (
	"fmt"
	"time"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/sync"
	"github.com/syncthing/syncthing/lib/versioner"
)

func init() {
	folderFactories[config.FolderTypeReadOnly] = newROFolder
}

type roFolder struct {
	folder
}

func newROFolder(model *Model, cfg config.FolderConfiguration, ver versioner.Versioner) service {
	return &roFolder{
		folder: folder{
			stateTracker: stateTracker{
				folderID: cfg.ID,
				mut:      sync.NewMutex(),
			},
			scan: folderscan{
				interval: time.Duration(cfg.RescanIntervalS) * time.Second,
				timer:    time.NewTimer(time.Millisecond),
				now:      make(chan rescanRequest),
				delay:    make(chan time.Duration),
			},
			stop:  make(chan struct{}),
			model: model,
		},
	}
}

func (f *roFolder) Serve() {
	l.Debugln(f, "starting")
	defer l.Debugln(f, "exiting")

	defer func() {
		f.scan.timer.Stop()
	}()

	initialScanCompleted := false
	for {
		select {
		case <-f.stop:
			return

		case <-f.scan.timer.C:
			if err := f.model.CheckFolderHealth(f.folderID); err != nil {
				l.Infoln("Skipping folder", f.folderID, "scan due to folder error:", err)
				f.scan.reschedule()
				continue
			}

			l.Debugln(f, "rescan")

			if err := f.model.internalScanFolderSubdirs(f.folderID, nil); err != nil {
				// Potentially sets the error twice, once in the scanner just
				// by doing a check, and once here, if the error returned is
				// the same one as returned by CheckFolderHealth, though
				// duplicate set is handled by setError.
				f.setError(err)
				f.scan.reschedule()
				continue
			}

			if !initialScanCompleted {
				l.Infoln("Completed initial scan (ro) of folder", f.folderID)
				initialScanCompleted = true
			}

			if f.scan.interval == 0 {
				continue
			}

			f.scan.reschedule()

		case req := <-f.scan.now:
			req.err <- f.scanSubdirsIfHealthy(req.subdirs)

		case next := <-f.scan.delay:
			f.scan.timer.Reset(next)
		}
	}
}

func (f *roFolder) String() string {
	return fmt.Sprintf("roFolder/%s@%p", f.folderID, f)
}
