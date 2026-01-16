// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/choria-io/ccm/internal/backoff"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/metrics"
)

// maintainHttpCache periodically checks an HTTP(S) URL for changes and updates the local cache.
// It uses HEAD requests with Last-Modified/ETag headers for change detection.
func (w *worker) maintainHttpCache(wg *sync.WaitGroup, manifestUrl *url.URL) {
	ticker := time.NewTicker(objectMaintInterval)
	w.fetchNotify = make(chan struct{}, 1)
	w.fetchNotify <- struct{}{} // Trigger initial fetch

	redactedUrl := iu.RedactUrlCredentials(manifestUrl)
	tries := 0

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-w.fetchNotify:
				changed, err := w.checkHttpChanged(manifestUrl)
				if err != nil {
					metrics.AgentManifestFetchFailureCount.WithLabelValues(w.source).Inc()
					tries++
					w.log.Error("Could not check HTTP manifest for changes", "url", redactedUrl, "error", err)
					backoff.Default.AfterFunc(tries, func() {
						select {
						case w.fetchNotify <- struct{}{}:
						default:
						}
					})
					continue
				}

				if !changed {
					w.log.Debug("HTTP manifest unchanged", "url", redactedUrl)
					tries = 0
					continue
				}

				err = w.getHttpFile(manifestUrl)
				if err != nil {
					metrics.AgentManifestFetchFailureCount.WithLabelValues(w.source).Inc()
					tries++
					w.log.Error("Could not fetch HTTP manifest", "url", redactedUrl, "error", err)
					backoff.Default.AfterFunc(tries, func() {
						select {
						case w.fetchNotify <- struct{}{}:
						default:
						}
					})
					continue
				}
				tries = 0

			case <-ticker.C:
				ticker.Reset(objectMaintInterval)
				select {
				case w.fetchNotify <- struct{}{}:
				default:
				}

			case <-w.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// checkHttpChanged performs a HEAD request to check if the HTTP resource has changed.
// Returns true if the resource needs to be fetched, false if unchanged.
func (w *worker) checkHttpChanged(manifestUrl *url.URL) (bool, error) {
	// If server doesn't support caching headers, we've already fetched once
	if w.httpNoCacheHeaders {
		return false, nil
	}

	// First fetch - no cached headers yet
	if w.httpLastModified == "" && w.httpETag == "" {
		return true, nil
	}

	timeoutCtx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodHead, manifestUrl.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	// Add Basic Auth if credentials are provided in the URL
	if manifestUrl.User != nil {
		username := manifestUrl.User.Username()
		password, _ := manifestUrl.User.Password()
		req.SetBasicAuth(username, password)
	}

	// Add conditional headers
	if w.httpLastModified != "" {
		req.Header.Set("If-Modified-Since", w.httpLastModified)
	}
	if w.httpETag != "" {
		req.Header.Set("If-None-Match", w.httpETag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// 304 - content hasn't changed
		return false, nil

	case http.StatusOK:
		// Check if headers indicate a change
		newLastModified := resp.Header.Get("Last-Modified")
		newETag := resp.Header.Get("ETag")

		// Check for changes based on headers
		if newETag != "" && newETag != w.httpETag {
			return true, nil
		}
		if newLastModified != "" && newLastModified != w.httpLastModified {
			return true, nil
		}

		// Headers match, no change
		return false, nil

	default:
		return false, fmt.Errorf("unexpected HTTP status: %d %s", resp.StatusCode, resp.Status)
	}
}

// getHttpFile downloads the manifest from an HTTP URL and caches it locally.
func (w *worker) getHttpFile(manifestUrl *url.URL) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	metrics.AgentManifestFetchCount.WithLabelValues(w.source).Inc()

	redactedUrl := iu.RedactUrlCredentials(manifestUrl)
	w.log.Warn("Fetching manifest from HTTP", "url", redactedUrl)

	timeoutCtx, cancel := context.WithTimeout(w.ctx, time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, manifestUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	// Add Basic Auth if credentials are provided in the URL
	if manifestUrl.User != nil {
		username := manifestUrl.User.Username()
		password, _ := manifestUrl.User.Password()
		req.SetBasicAuth(username, password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Store cache validation headers for next check
	w.httpLastModified = resp.Header.Get("Last-Modified")
	w.httpETag = resp.Header.Get("ETag")

	// Check if server supports caching headers
	if w.httpLastModified == "" && w.httpETag == "" {
		w.httpNoCacheHeaders = true
		w.log.Warn("HTTP server does not support Last-Modified or ETag headers, change detection disabled", "url", redactedUrl)
	}

	// Create temp directory for extraction
	cleanedPath := w.cleanHttpPath(manifestUrl)
	tf, err := os.MkdirTemp(w.cacheDir, fmt.Sprintf("http-%s.*", cleanedPath))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tf)

	// Extract tar.gz directly from response body (streaming)
	files, err := iu.UntarGz(resp.Body, tf)
	if err != nil {
		return err
	}

	// Find manifest.yaml
	manifestPath, err := iu.FindManifestInFiles(files, tf)
	if err != nil {
		return fmt.Errorf("%w in fetched file", err)
	}

	// Atomic rename to final location
	target := filepath.Join(w.cacheDir, fmt.Sprintf("http_%s", cleanedPath))
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

	w.log.Debug("HTTP manifest cached", "target", target, "manifest", w.manifestPath)
	w.mgr.SetWorkingDirectory(filepath.Dir(w.manifestPath))

	w.triggerApply()

	return nil
}

// cleanHttpPath creates a filesystem-safe identifier from a URL.
func (w *worker) cleanHttpPath(u *url.URL) string {
	path := u.Host + u.Path
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, ":", "_")
	path = strings.TrimSuffix(path, ".tar.gz")
	path = strings.TrimSuffix(path, ".tgz")
	return path
}
