// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model/modelmocks"
)

// Tests are run via TestConfig in config_test.go

var _ = Describe("cleanFileName", func() {
	It("removes .tar.gz extension", func() {
		w := &worker{}
		result := w.cleanFileName("manifest.tar.gz")
		Expect(result).To(Equal("manifest"))
	})

	It("removes .tgz extension", func() {
		w := &worker{}
		result := w.cleanFileName("manifest.tgz")
		Expect(result).To(Equal("manifest"))
	})

	It("handles files without extension", func() {
		w := &worker{}
		result := w.cleanFileName("manifest")
		Expect(result).To(Equal("manifest"))
	})

	It("handles files with other extensions", func() {
		w := &worker{}
		result := w.cleanFileName("manifest.yaml")
		Expect(result).To(Equal("manifest.yaml"))
	})

	It("handles nested .tar.gz extensions", func() {
		w := &worker{}
		result := w.cleanFileName("manifest.tar.gz.tar.gz")
		Expect(result).To(Equal("manifest.tar.gz"))
	})
})

var _ = Describe("cacheManifest with obj://", func() {
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

	It("routes obj:// URLs to maintainObjectCache", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Allow JetStream calls from the goroutine (will fail, causing retry loop)
		mockMgr.EXPECT().JetStream().Return(nil, context.DeadlineExceeded).AnyTimes()

		w = &worker{
			source: "obj://mybucket/manifest.tar.gz",
			ctx:    ctx,
			log:    mockLog,
			mgr:    mockMgr,
		}

		var wg sync.WaitGroup
		err := w.cacheManifest(&wg)
		Expect(err).NotTo(HaveOccurred())

		// maintainObjectCache was launched as goroutine
		// The JetStream call will fail, but routing worked
	})

	It("returns error for unsupported schemes", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w = &worker{
			source: "ftp://example.com/manifest.tar.gz",
			ctx:    ctx,
			log:    mockLog,
			mgr:    mockMgr,
		}

		var wg sync.WaitGroup
		err := w.cacheManifest(&wg)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported manifest source"))
	})
})

var _ = Describe("setJetStream", func() {
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
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	It("returns false when JetStream connection fails", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w = &worker{
			ctx: ctx,
			log: mockLog,
			mgr: mockMgr,
		}

		mockMgr.EXPECT().JetStream().Return(nil, context.DeadlineExceeded)

		result := w.setJetStream()
		Expect(result).To(BeFalse())
		Expect(w.js).To(BeNil())
	})
})
