// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package goss

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/choria-io/ccm/internal/backoff"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/templates"
	"github.com/goss-org/goss"
	"github.com/goss-org/goss/outputs"
	gossutil "github.com/goss-org/goss/util"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ErrRulesNotSpecified = errors.New("check rules not specified")
)

func Execute(ctx context.Context, mgr model.Manager, hc *model.CommonHealthCheck, userLogger model.Logger, log model.Logger) (*model.HealthCheckResult, error) {
	if len(hc.GossRules) == 0 {
		return nil, ErrRulesNotSpecified
	}

	var result model.HealthCheckResult

	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	tf, err := os.CreateTemp("", "goss-*.yaml")
	if err != nil {
		return nil, err
	}

	rules, err := templates.ResolveTemplateString(string(hc.GossRules), env)
	if err != nil {
		return nil, err
	}

	_, err = tf.WriteString(rules)
	if err != nil {
		tf.Close()
		return nil, err
	}
	err = tf.Close()
	if err != nil {
		return nil, err
	}

	defer os.Remove(tf.Name())

	var out bytes.Buffer
	opts := []gossutil.ConfigOption{
		gossutil.WithMaxConcurrency(1),
		gossutil.WithResultWriter(&out),
		gossutil.WithSpecFile(tf.Name()),
	}

	cfg, err := gossutil.NewConfig(opts...)
	if err != nil {
		return nil, err
	}

	tries := hc.Tries
	if tries < 1 {
		tries = 1
	}

	if userLogger != nil {
		userLogger = userLogger.With("format", "goss")
	}

	for attempt := 1; attempt <= tries; attempt++ {
		// Check if context is canceled before attempting
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		log.Info("Executing goss check", "try", attempt)
		out.Reset()

		result.Tries = attempt

		timer := prometheus.NewTimer(metrics.HealthCheckTime.WithLabelValues(hc.TypeName, hc.ResourceName, hc.Name))
		_, err = goss.Validate(cfg)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}

		results := &outputs.StructuredOutput{}
		err = json.Unmarshal(out.Bytes(), &results)
		if err != nil {
			return nil, err
		}

		result.Output = results.SummaryLine
		var ok bool
		if results.Summary.Failed > 0 {
			result.Status = model.HealthCheckCritical
		} else {
			ok = true
			result.Status = model.HealthCheckOK
		}

		if userLogger != nil {
			for _, res := range results.Results {
				if res.Result == 0 {
					userLogger.Debug(res.SummaryLineCompact)
				} else {
					userLogger.Error(res.SummaryLineCompact)
				}

			}
			if results.Summary.Failed > 0 {
				userLogger.Error("Health check failed", "failed", results.Summary.Failed, "msg", results.SummaryLine)
			} else {
				userLogger.Debug("Health check passed", "result", results.SummaryLine, "msg", results.SummaryLine)
			}
		}

		if ok {
			break
		}

		// If this was the last attempt, return the result
		if attempt >= tries {
			break
		}

		sleep := hc.ParseTrySleep
		if sleep == 0 {
			sleep = time.Second
		}

		err = backoff.InterruptableSleep(ctx, hc.ParseTrySleep)
		if err != nil {
			return nil, err
		}
	}

	metrics.HealthStatusCount.WithLabelValues(hc.TypeName, hc.ResourceName, result.Status.String(), hc.Name).Inc()

	return &result, nil
}
