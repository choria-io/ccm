// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/choria-io/ccm/hiera"
	"github.com/choria-io/ccm/internal/backoff"
	"github.com/choria-io/ccm/internal/facts"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
)

const DefaultInterval = 5 * time.Minute
const MinInterval = 30 * time.Second
const DefaultMaxDataRefreshTries = 10
const DefaultCacheDir = "/etc/choria/ccm/source"
const MinFactUpdateInterval = 2 * time.Minute

type Agent struct {
	mgr               model.Manager
	cfg               *Config
	log               model.Logger
	started           bool
	workers           map[string]*worker
	refreshTries      int
	previousFacts     map[string]any
	previousFactsTime time.Time
	previousData      map[string]any
	applyTrigger      chan *worker

	ctx    context.Context
	cancel context.CancelFunc

	wwg sync.WaitGroup

	mu sync.Mutex
}

// TODO: watch kv and only re-fetch data if it changes, but resolve each time for facts updates

// New creates a new agent
func New(cfg *Config, opts ...Option) (*Agent, error) {
	logger, err := cfg.NewLogger()
	if err != nil {
		return nil, err
	}

	natsTarget := cfg.NatsContext
	var natsProvider model.NatsConnProvider = &cachingNatsProvider{}
	if cfg.ChoriaTokenFile != "" || cfg.ChoriaSeedFile != "" {
		natsTarget = cfg.NatsServers
		natsProvider = newChoriaNatsProvider(cfg)
	}

	mgr, err := manager.NewManager(logger, logger, manager.WithNatsContext(natsTarget), manager.WithNatsConnection(natsProvider))
	if err != nil {
		return nil, err
	}

	a := &Agent{
		mgr:          mgr,
		log:          logger,
		cfg:          cfg,
		applyTrigger: make(chan *worker, 1),
		refreshTries: DefaultMaxDataRefreshTries,
	}

	cfg.intervalDuration = DefaultInterval

	for _, opt := range opts {
		err := opt(a)
		if err != nil {
			return nil, err
		}
	}

	if a.cfg.CacheDir == "" {
		a.cfg.CacheDir = DefaultCacheDir
	}
	err = os.MkdirAll(a.cfg.CacheDir, 0755)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Agent) Run(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()

	a.log.Warn("Starting agent", "interval", a.cfg.intervalDuration, "health_interval", a.cfg.healthCheckIntervalDuration, "manifests", len(a.cfg.Manifests))

	if a.cfg.MonitorPort > 0 {
		metrics.RegisterMetrics()
		metrics.ListenAndServe(a.cfg.MonitorPort, a.log)
	}

	a.mu.Lock()

	if a.started {
		a.mu.Unlock()
		return fmt.Errorf("already started")
	}

	a.ctx, a.cancel = context.WithCancel(ctx)
	defer a.cancel()
	a.started = true

	var err error

	// workers before data so data can be set in the workers
	a.workers, err = a.createWorkers()
	if err != nil {

		return err
	}

	if len(a.workers) == 0 {
		return fmt.Errorf("no manifests configured")
	}

	// do this once at start so object store watcher based apply
	// triggers already have correct data
	a.updateData()

	a.mu.Unlock()

	// data is ready lets start them
	for _, w := range a.workers {
		a.wwg.Add(1)
		go w.start(a.ctx, &a.wwg)
	}

	// we have a single ticker outside all the workers to simplify scheduling
	// facts refreshes, conflicts between applies etc. The worker maintains object caches
	// and triggers apply based on this ticker.
	//
	// workers do trigger applies though after object store updates, these will schedule
	// in between the generally scheduled apply cycle
	applyTicker := time.NewTicker(a.cfg.intervalDuration)

	healthCheckTicker := time.NewTicker(math.MaxInt64)
	if a.cfg.healthCheckIntervalDuration > 0 {
		healthCheckTicker.Reset(a.cfg.healthCheckIntervalDuration)
	}

	for {
		select {
		case w := <-a.applyTrigger:
			a.mu.Lock()
			a.updateData()
			w.apply(false, true) // we force it to run even if it was recently ran as this channel indicates a priority run is needed
			a.mu.Unlock()

		case <-healthCheckTicker.C:
			a.runHealthChecks()

		case <-applyTicker.C:
			a.runManifests()

		case <-a.ctx.Done():
			a.cancel()
			a.wwg.Wait()

			a.log.Warn("Runner stopped")

			return nil
		}
	}
}

func (a *Agent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cancel()

	a.wwg.Wait()

	a.mgr.Close()
	a.started = false
	a.ctx = nil
	a.cancel = nil

	return nil
}

func (a *Agent) updateData() {
	a.getFacts(a.ctx)
	a.getData(a.ctx)
}

func (a *Agent) runManifests() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Info("Starting scheduled run", "workers", len(a.workers))

	a.updateData()

	for _, w := range a.workers {
		w.apply(false, false)
	}

	a.log.Info("Completed scheduled run")
}

// health checks do not refresh facts or data
func (a *Agent) runHealthChecks() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Info("Running health checks", "workers", len(a.workers))

	wg := sync.WaitGroup{}
	var triggers []func()

	for _, w := range a.workers {
		wg.Add(1)
		go func(wg *sync.WaitGroup, w *worker) {
			defer wg.Done()

			report := w.apply(true, false)
			if report != nil && report.HealthCheckCriticalCount > 0 {
				a.log.Error("Healthcheck reported critical, triggering apply to remediate", "critical", report.HealthCheckCriticalCount)
				metrics.AgentHealthCheckRemediation.WithLabelValues(w.source).Inc()
				triggers = append(triggers, w.triggerApply)
			}
		}(&wg, w)
	}

	wg.Wait()

	// avoid applies and checks interleaving
	for _, t := range triggers {
		t()
	}

	a.log.Info("Completed scheduled health check run")

}

// lock must be hald before calling
func (a *Agent) getFacts(ctx context.Context) {
	if a.previousFacts != nil && time.Since(a.previousFactsTime) < MinFactUpdateInterval {
		a.log.Debug(fmt.Sprintf("Skipping facts refresh, last refresh was less than %v ago", MinFactUpdateInterval))
		return
	}

	timer := prometheus.NewTimer(metrics.AgentFactsResolveTime.WithLabelValues())
	defer timer.ObserveDuration()

	backoff.Default.For(ctx, func(try int) error {
		log := a.log.With("try", try)

		if try > a.refreshTries && a.previousFacts != nil {
			log.Info("Using previous facts after repeated failures")
			a.mgr.SetFacts(a.previousFacts)
			for _, w := range a.workers {
				w.setFacts(a.previousFacts)
			}

			return nil
		}

		log.Info("Refreshing facts")
		f, err := facts.StandardFacts(ctx, a.log)
		if err != nil {
			log.Error("Could not get system facts", "error", err)
			metrics.AgentFactsResolveFailureCount.WithLabelValues().Inc()
			return err
		}

		// we always set it so the mgr in the workers dont start doing it, they check for nil
		a.mgr.SetFacts(f)
		for _, w := range a.workers {
			w.setFacts(f)
		}
		a.previousFacts = f

		return nil
	})
}

func (a *Agent) getData(ctx context.Context) {
	if a.cfg.ExternalDataUrl == "" {
		a.log.Debug("Skipping external data resolution, no url configured")
		return
	}

	timer := prometheus.NewTimer(metrics.AgentDataResolveTime.WithLabelValues())
	defer timer.ObserveDuration()

	backoff.Default.For(ctx, func(try int) error {
		log := a.log.With("try", try)

		if try > a.refreshTries && a.previousData != nil {
			log.Info("Using previous data after repeated failures")
			a.mgr.SetData(a.previousData)
			for _, w := range a.workers {
				w.setExternalData(a.previousData)
			}
			return nil
		}

		log.Info("Refreshing external data")
		f, err := a.mgr.Facts(ctx)
		if err != nil {
			log.Error("Could not get facts to resolve data", "error", err)
			metrics.AgentDataResolveFailureCount.WithLabelValues(a.cfg.ExternalDataUrl).Inc()
			return err
		}

		resolved, err := hiera.ResolveUrl(ctx, a.cfg.ExternalDataUrl, a.mgr, f, hiera.DefaultOptions, a.log)
		if err != nil {
			log.Error("Could not resolve external data", "error", err)
			metrics.AgentDataResolveFailureCount.WithLabelValues(a.cfg.ExternalDataUrl).Inc()
			return err
		}

		if len(resolved) == 0 {
			log.Warn("External data resolved to empty map")
			return fmt.Errorf("empty external data, cannot continue")
		}

		a.mgr.SetData(resolved)
		for _, w := range a.workers {
			w.setExternalData(resolved)
		}

		a.previousData = resolved

		log.Debug("Gathered external data", "items", len(resolved))

		return nil
	})
}

func (a *Agent) createWorkers() (map[string]*worker, error) {
	workers := make(map[string]*worker)

	for _, v := range a.cfg.Manifests {
		// each needs a manager as manager handles sessions etc
		// later when doing wath for data or regular fact freshes
		// we need to update them all
		mgr, err := manager.NewManager(a.log, a.log)
		if err != nil {
			return nil, err
		}
		err = mgr.CopyFrom(a.mgr)
		if err != nil {
			return nil, err
		}

		workers[v] = &worker{
			source:            v,
			mgr:               mgr,
			cacheDir:          a.cfg.CacheDir,
			agentApplyTrigger: a.applyTrigger,
			log:               a.log,
		}
	}

	return workers, nil
}
