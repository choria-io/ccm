// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/choria-io/ccm/internal/backoff"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/apply"
)

const objectMaintInterval = 30 * time.Second

type worker struct {
	source            string
	ctx               context.Context
	cancel            context.CancelCauseFunc
	mgr               model.Manager
	cacheDir          string
	log               model.Logger
	js                jetstream.JetStream
	manifestPath      string
	fetchNotify       chan struct{}
	applyNotify       chan struct{}
	facts             map[string]any
	externalData      map[string]any
	lastApply         time.Time
	agentApplyTrigger chan *worker

	mu sync.Mutex
}

func (w *worker) start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	w.ctx, w.cancel = context.WithCancelCause(ctx)
	defer w.cancel(nil)

	w.applyNotify = make(chan struct{}, 1)
	err := w.cacheManifest(wg)
	if err != nil {
		w.log.Error("Could not start manifest cache maintainer", "error", err)
		return
	}

	for {
		select {
		case <-w.applyNotify:
			w.agentApplyTrigger <- w

		case <-ctx.Done():
			w.mgr.Close()
			cause := context.Cause(ctx)
			if cause != nil {
				w.log.Warn("Shutting down due to context cancellation", "cause", cause)
			} else {
				w.log.Warn("Shutting down")
			}
			return
		}
	}
}

func (w *worker) cacheManifest(wg *sync.WaitGroup) error {
	uri, err := url.Parse(w.source)
	if err != nil {
		return err
	}

	switch uri.Scheme {
	case "obj":
		f := strings.TrimPrefix(uri.Path, "/")
		w.log.Info("Maintaining object cache for manifest", "bucket", uri.Host, "file", f)
		go w.maintainObjectCache(wg, uri.Host, f)

	case "":
		w.manifestPath = w.source
		w.mgr.SetWorkingDirectory(filepath.Dir(w.manifestPath))
		w.triggerApply()

	default:
		return fmt.Errorf("unsupported manifest source: %s", w.source)
	}

	return nil
}

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

	var manifestPath string
	for _, f := range files {
		if filepath.Base(f) == "manifest.yaml" {
			manifestPath = strings.TrimPrefix(f, tf)
			break
		}
	}
	if manifestPath == "" {
		return fmt.Errorf("manifest.yaml not found in fetched file")
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

func (w *worker) getBucket(bucket string) (jetstream.ObjectStore, bool) {
	obj, err := w.js.ObjectStore(w.ctx, bucket)
	if err != nil {
		w.log.Error("Could not connect to JetStream bucket", "bucket", bucket, "error", err)
		return nil, false
	}

	return obj, true
}

func (w *worker) setJetStream() bool {
	var err error

	w.js, err = w.mgr.JetStream()
	if err != nil {
		w.log.Error("Could not connect to JetStream", "error", err)
		return false
	}

	return true
}

func (w *worker) triggerApply() {
	select {
	case w.applyNotify <- struct{}{}:
	default:
	}
}

func (w *worker) apply(hcOnly bool, force bool) *model.SessionSummary {
	w.mu.Lock()
	defer w.mu.Unlock()

	timer := prometheus.NewTimer(metrics.AgentApplyTime.WithLabelValues(w.source))
	timer.ObserveDuration()

	if w.manifestPath == "" {
		w.log.Warn("No manifest path set, skipping apply")
	}

	log := w.log
	if w.manifestPath != w.source {
		log = w.log.With("manifest", w.manifestPath)
	}

	if !force && !hcOnly && time.Since(w.lastApply) < MinInterval {
		log.Warn("Skipping apply due to minimum configured apply interval", "since", time.Since(w.lastApply).Round(time.Second), "min", MinInterval)
		return nil
	}

	if hcOnly {
		log = log.With("healthcheck", true)
	} else {
		w.lastApply = time.Now()
	}

	manifestPath := w.manifestPath

	// before apply make sure we have the latest facts and data
	// as this is managed externally by a timer and watcher
	w.mgr.SetFacts(w.facts)

	_, manifest, err := apply.ResolveManifestFilePath(w.ctx, w.mgr, manifestPath, apply.WithOverridingResolvedData(w.externalData))
	if err != nil {
		log.Error("Could not resolve manifest", "error", err)
		return nil
	}

	_, err = manifest.Execute(w.ctx, w.mgr, hcOnly, log)
	if err != nil {
		log.Error("Could not execute manifest", "error", err)
		return nil
	}

	report, err := w.mgr.SessionSummary()
	if err != nil {
		w.log.Error("Could not get session summary", "error", err)
		return nil
	}

	switch {
	case report.ChangedResources > 0:
		log.Warn(report.String())
	case report.FailedResources > 0:
		log.Error(report.String())
	default:
		log.Info(report.String())
	}

	return report
}

func (w *worker) cleanFileName(file string) string {
	file = strings.TrimSuffix(file, ".tar.gz")
	file = strings.TrimSuffix(file, ".tgz")

	return file
}

func (w *worker) setFacts(facts map[string]any) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.log.Debug("Setting facts", "items", len(facts))

	w.facts = iu.CloneMap(facts)
}

func (w *worker) setExternalData(data map[string]any) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.log.Debug("Setting external data", "items", len(data))
	w.externalData = iu.CloneMap(data)
}
