// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/choria-io/ccm/internal/backoff"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/metrics"
)

// maintainObjectCache periodically checks a JetStream Object Store for changes and updates the local cache.
// It uses JetStream's native object watcher for real-time change detection.
func (w *worker) maintainObjectCache(wg *sync.WaitGroup, bucket string, file string) {
	ticker := time.NewTicker(objectMaintInterval)
	updateNotify := make(chan struct{}, 1)
	updateNotify <- struct{}{}
	var watch jetstream.ObjectWatcher
	var obj jetstream.ObjectStore

	for {
		select {
		case <-updateNotify:
			w.log.Debug("Checking for updates to JetStream bucket", "bucket", bucket, "file", file)
			if w.js == nil {
				if !w.setJetStream() {
					continue
				}
			}

			if obj == nil {
				var ok bool
				obj, ok = w.getBucket(bucket)
				if !ok {
					continue
				}
			}

			if watch == nil {
				watch, _ = w.watchFile(wg, obj, bucket, file)
			}

		case <-ticker.C:
			ticker.Reset(objectMaintInterval)
			updateNotify <- struct{}{}

		case <-w.ctx.Done():
			ticker.Stop()
			if watch != nil {
				watch.Stop()
			}
		}
	}
}

// watchFile creates a watcher for changes to a specific file in the object store.
func (w *worker) watchFile(wg *sync.WaitGroup, obj jetstream.ObjectStore, bucket string, file string) (jetstream.ObjectWatcher, bool) {
	watch, err := obj.Watch(w.ctx, jetstream.UpdatesOnly())
	if err != nil {
		w.log.Error("Could not watch JetStream bucket", "error", err)
		return nil, false
	}

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		w.fetchNotify = make(chan struct{}, 1)
		w.fetchNotify <- struct{}{}

		tries := 0

		for {
			select {
			case <-w.fetchNotify:
				err := w.getFile(obj, bucket, file)
				if err != nil {
					metrics.AgentManifestFetchFailureCount.WithLabelValues(w.source).Inc()
					tries++
					w.log.Error("Could not fetch file from JetStream bucket", "error", err)
					backoff.Default.AfterFunc(tries, func() {
						select {
						case w.fetchNotify <- struct{}{}:
						default:
						}
					})
					continue
				}
				tries = 0

			case nfo := <-watch.Updates():
				if nfo == nil {
					continue
				}

				if nfo.Name != file {
					continue
				}

				if nfo.Deleted {
					w.log.Warn("File deleted from JetStream bucket, terminating management")
					w.cancel(fmt.Errorf("file deleted from bucket"))
					continue
				}

				select {
				case w.fetchNotify <- struct{}{}:
				default:
				}

			case <-w.ctx.Done():
				watch.Stop() // should not be needed so no need to log errors
				return
			}
		}
	}(wg)

	return watch, true
}

// getFile downloads a file from the object store and caches it locally.
func (w *worker) getFile(obj jetstream.ObjectStore, bucket string, file string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	metrics.AgentManifestFetchCount.WithLabelValues(w.source).Inc()

	w.log.Warn("Fetching file from JetStream bucket", "bucket", bucket, "file", file)
	f, err := obj.Get(context.TODO(), file)
	if err == nil {
		err = f.Error()
	}
	if err != nil {
		return fmt.Errorf("get failed: %w", err)
	}

	nfo, err := f.Info()
	if err != nil {
		return fmt.Errorf("info failed: %w", err)
	}
	w.log.Info("File updated in JetStream bucket, fetching", "size", nfo.Size)

	tf, err := os.MkdirTemp(w.cacheDir, fmt.Sprintf("%s-%s.*", bucket, w.cleanFileName(file)))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tf)

	files, err := iu.UntarGz(f, tf)
	if err != nil {
		return err
	}

	manifestPath, err := iu.FindManifestInFiles(files, tf)
	if err != nil {
		return fmt.Errorf("%w in fetched file", err)
	}

	target := filepath.Join(w.cacheDir, fmt.Sprintf("%s_%s", bucket, w.cleanFileName(file)))
	if iu.FileExists(target) {
		err = os.RemoveAll(target)
		if err != nil {
			return err
		}
	}

	err = os.Rename(tf, target)
	if err != nil {
		return err
	}

	w.manifestPath = filepath.Join(target, manifestPath)

	w.log.Debug("File moved to cache", "target", target, "manifest", w.manifestPath)
	w.mgr.SetWorkingDirectory(filepath.Dir(w.manifestPath))

	w.triggerApply()

	return nil
}

// getBucket gets an object store bucket from JetStream.
func (w *worker) getBucket(bucket string) (jetstream.ObjectStore, bool) {
	obj, err := w.js.ObjectStore(w.ctx, bucket)
	if err != nil {
		w.log.Error("Could not connect to JetStream bucket", "bucket", bucket, "error", err)
		return nil, false
	}

	return obj, true
}

// setJetStream initializes the JetStream connection.
func (w *worker) setJetStream() bool {
	var err error

	w.js, err = w.mgr.JetStream()
	if err != nil {
		w.log.Error("Could not connect to JetStream", "error", err)
		return false
	}

	return true
}

// cleanFileName removes tar.gz/tgz extensions from a filename.
func (w *worker) cleanFileName(file string) string {
	file = strings.TrimSuffix(file, ".tar.gz")
	file = strings.TrimSuffix(file, ".tgz")

	return file
}
