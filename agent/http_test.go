// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model/modelmocks"
)

// Tests are run via TestConfig in config_test.go

// createTarGz creates a tar.gz archive containing a manifest.yaml file
func createTarGz(manifestContent string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add manifest.yaml
	hdr := &tar.Header{
		Name: "manifest.yaml",
		Mode: 0644,
		Size: int64(len(manifestContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(manifestContent)); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

var _ = Describe("cleanHttpPath", func() {
	It("creates a filesystem-safe path from URL", func() {
		w := &worker{}

		u, _ := url.Parse("https://example.com:8443/path/to/manifest.tar.gz")
		result := w.cleanHttpPath(u)
		Expect(result).To(Equal("example.com_8443_path_to_manifest"))
	})

	It("handles URLs without port", func() {
		w := &worker{}

		u, _ := url.Parse("https://example.com/manifest.tgz")
		result := w.cleanHttpPath(u)
		Expect(result).To(Equal("example.com_manifest"))
	})

	It("handles URLs with multiple path segments", func() {
		w := &worker{}

		u, _ := url.Parse("https://cdn.example.com/releases/v1.0/app.tar.gz")
		result := w.cleanHttpPath(u)
		Expect(result).To(Equal("cdn.example.com_releases_v1.0_app"))
	})
})

var _ = Describe("checkHttpChanged", func() {
	var (
		mockctl *gomock.Controller
		mockLog *modelmocks.MockLogger
		mockMgr *modelmocks.MockManager
		w       *worker
		server  *httptest.Server
		ctx     context.Context
		cancel  context.CancelFunc
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(mockctl)
		mockMgr = modelmocks.NewMockManager(mockctl)
		ctx, cancel = context.WithCancel(context.Background())

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

		w = &worker{
			ctx: ctx,
			log: mockLog,
			mgr: mockMgr,
		}
	})

	AfterEach(func() {
		cancel()
		mockctl.Finish()
		if server != nil {
			server.Close()
		}
	})

	It("returns true on first check (no cached headers)", func() {
		u, _ := url.Parse("https://example.com/manifest.tar.gz")
		changed, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("returns false when httpNoCacheHeaders is true", func() {
		w.httpNoCacheHeaders = true
		u, _ := url.Parse("https://example.com/manifest.tar.gz")
		changed, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("returns false when server returns 304 Not Modified", func() {
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				if r.Header.Get("If-None-Match") == `"etag123"` {
					rw.WriteHeader(http.StatusNotModified)
					return
				}
			}
			rw.WriteHeader(http.StatusOK)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		w.httpETag = `"etag123"`

		changed, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("returns true when ETag changes", func() {
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("ETag", `"new-etag"`)
			rw.WriteHeader(http.StatusOK)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		w.httpETag = `"old-etag"`

		changed, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("returns true when Last-Modified changes", func() {
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("Last-Modified", "Wed, 15 Jan 2026 12:00:00 GMT")
			rw.WriteHeader(http.StatusOK)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		w.httpLastModified = "Wed, 01 Jan 2026 12:00:00 GMT"

		changed, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
	})

	It("returns false when headers match", func() {
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("ETag", `"same-etag"`)
			rw.Header().Set("Last-Modified", "Wed, 01 Jan 2026 12:00:00 GMT")
			rw.WriteHeader(http.StatusOK)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		w.httpETag = `"same-etag"`
		w.httpLastModified = "Wed, 01 Jan 2026 12:00:00 GMT"

		changed, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
	})

	It("returns error for HTTP errors", func() {
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		w.httpETag = `"some-etag"`

		_, err := w.checkHttpChanged(u)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("500"))
	})

	It("sends Basic Auth when URL has credentials", func() {
		var receivedAuth string
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			rw.Header().Set("ETag", `"new-etag"`)
			rw.WriteHeader(http.StatusOK)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		u.User = url.UserPassword("testuser", "testpass")
		w.httpETag = `"old-etag"`

		_, err := w.checkHttpChanged(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(receivedAuth).To(HavePrefix("Basic "))
	})
})

var _ = Describe("getHttpFile", func() {
	var (
		mockctl  *gomock.Controller
		mockLog  *modelmocks.MockLogger
		mockMgr  *modelmocks.MockManager
		w        *worker
		server   *httptest.Server
		ctx      context.Context
		cancel   context.CancelFunc
		cacheDir string
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(mockctl)
		mockMgr = modelmocks.NewMockManager(mockctl)
		ctx, cancel = context.WithCancel(context.Background())

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		cacheDir, err = os.MkdirTemp("", "http-cache-test-*")
		Expect(err).NotTo(HaveOccurred())

		w = &worker{
			ctx:         ctx,
			log:         mockLog,
			mgr:         mockMgr,
			cacheDir:    cacheDir,
			applyNotify: make(chan struct{}, 1),
		}
	})

	AfterEach(func() {
		cancel()
		mockctl.Finish()
		if server != nil {
			server.Close()
		}
		os.RemoveAll(cacheDir)
	})

	It("downloads and extracts manifest successfully", func() {
		manifestContent := `
name: test
resources: []
`
		tarGz, err := createTarGz(manifestContent)
		Expect(err).NotTo(HaveOccurred())

		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("ETag", `"test-etag"`)
			rw.Header().Set("Last-Modified", "Wed, 15 Jan 2026 12:00:00 GMT")
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(tarGz.Bytes())
		}))

		mockMgr.EXPECT().SetWorkingDirectory(gomock.Any()).Times(1)

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		err = w.getHttpFile(u)
		Expect(err).NotTo(HaveOccurred())

		// Verify cached headers
		Expect(w.httpETag).To(Equal(`"test-etag"`))
		Expect(w.httpLastModified).To(Equal("Wed, 15 Jan 2026 12:00:00 GMT"))

		// Verify manifest path is set
		Expect(w.manifestPath).NotTo(BeEmpty())
		Expect(filepath.Base(w.manifestPath)).To(Equal("manifest.yaml"))

		// Verify file exists
		_, err = os.Stat(w.manifestPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("sets httpNoCacheHeaders when server has no cache headers", func() {
		manifestContent := `
name: test
resources: []
`
		tarGz, err := createTarGz(manifestContent)
		Expect(err).NotTo(HaveOccurred())

		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(tarGz.Bytes())
		}))

		mockMgr.EXPECT().SetWorkingDirectory(gomock.Any()).Times(1)

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		err = w.getHttpFile(u)
		Expect(err).NotTo(HaveOccurred())

		Expect(w.httpNoCacheHeaders).To(BeTrue())
	})

	It("returns error when manifest.yaml is missing", func() {
		// Create tar.gz without manifest.yaml
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		hdr := &tar.Header{
			Name: "other.yaml",
			Mode: 0644,
			Size: 4,
		}
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write([]byte("test"))
		_ = tw.Close()
		_ = gw.Close()

		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(buf.Bytes())
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		err := w.getHttpFile(u)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("manifest.yaml not found"))
	})

	It("returns error for non-200 status codes", func() {
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		}))

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		err := w.getHttpFile(u)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("404"))
	})

	It("sends Basic Auth when URL has credentials", func() {
		manifestContent := `
name: test
resources: []
`
		tarGz, err := createTarGz(manifestContent)
		Expect(err).NotTo(HaveOccurred())

		var receivedAuth string
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			rw.Header().Set("ETag", `"test-etag"`)
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(tarGz.Bytes())
		}))

		mockMgr.EXPECT().SetWorkingDirectory(gomock.Any()).Times(1)

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		u.User = url.UserPassword("testuser", "testpass")

		err = w.getHttpFile(u)
		Expect(err).NotTo(HaveOccurred())
		Expect(receivedAuth).To(HavePrefix("Basic "))
	})

	It("triggers apply after successful download", func() {
		manifestContent := `
name: test
resources: []
`
		tarGz, err := createTarGz(manifestContent)
		Expect(err).NotTo(HaveOccurred())

		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("ETag", `"test-etag"`)
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(tarGz.Bytes())
		}))

		mockMgr.EXPECT().SetWorkingDirectory(gomock.Any()).Times(1)

		u, _ := url.Parse(server.URL + "/manifest.tar.gz")
		err = w.getHttpFile(u)
		Expect(err).NotTo(HaveOccurred())

		// Check that apply was triggered
		select {
		case <-w.applyNotify:
			// Good, apply was triggered
		case <-time.After(100 * time.Millisecond):
			Fail("Expected apply to be triggered")
		}
	})
})

var _ = Describe("cacheManifest with HTTP", func() {
	var (
		mockctl *gomock.Controller
		mockLog *modelmocks.MockLogger
		mockMgr *modelmocks.MockManager
		w       *worker
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(mockctl)
		mockMgr = modelmocks.NewMockManager(mockctl)

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().With(gomock.Any()).Return(mockLog).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	It("routes http:// URLs to maintainHttpCache", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w = &worker{
			source: "http://example.com/manifest.tar.gz",
			ctx:    ctx,
			log:    mockLog,
			mgr:    mockMgr,
		}

		var wg sync.WaitGroup
		err := w.cacheManifest(&wg)
		Expect(err).NotTo(HaveOccurred())

		// Give the goroutine time to start and create the channel
		Eventually(func() chan struct{} {
			return w.fetchNotify
		}, 100*time.Millisecond).ShouldNot(BeNil())
	})

	It("routes https:// URLs to maintainHttpCache", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w = &worker{
			source: "https://example.com/manifest.tar.gz",
			ctx:    ctx,
			log:    mockLog,
			mgr:    mockMgr,
		}

		var wg sync.WaitGroup
		err := w.cacheManifest(&wg)
		Expect(err).NotTo(HaveOccurred())

		// Give the goroutine time to start and create the channel
		Eventually(func() chan struct{} {
			return w.fetchNotify
		}, 100*time.Millisecond).ShouldNot(BeNil())
	})
})
