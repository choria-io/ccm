// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

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

	// HTTP cache state
	httpLastModified   string
	httpETag           string
	httpNoCacheHeaders bool

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
		w.log = w.log.With("bucket", uri.Host, "file", f)
		w.log.Info("Maintaining object cache for manifest")
		go w.maintainObjectCache(wg, uri.Host, f)

	case "http", "https":
		redactedUrl := iu.RedactUrlCredentials(uri)
		w.log = w.log.With("url", redactedUrl)
		w.log.Info("Maintaining HTTP cache for manifest")
		go w.maintainHttpCache(wg, uri)

	case "":
		w.manifestPath = w.source
		w.mgr.SetWorkingDirectory(filepath.Dir(w.manifestPath))
		w.triggerApply()

	default:
		return fmt.Errorf("unsupported manifest source: %s", w.source)
	}

	return nil
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
