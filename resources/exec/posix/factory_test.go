// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

var _ = Describe("Factory", func() {
	var (
		mockctl *gomock.Controller
		logger  *modelmocks.MockLogger
		runner  *modelmocks.MockCommandRunner
		f       *factory
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

		f = &factory{}
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("TypeName", func() {
		It("Should return exec type name", func() {
			Expect(f.TypeName()).To(Equal(model.ExecTypeName))
		})
	})

	Describe("Name", func() {
		It("Should return provider name", func() {
			Expect(f.Name()).To(Equal(ProviderName))
		})
	})

	Describe("New", func() {
		It("Should create a new provider with runner", func() {
			provider, err := f.New(logger, runner)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).ToNot(BeNil())
			Expect(provider.Name()).To(Equal(ProviderName))
		})

		It("Should create a provider with nil runner and warn", func() {
			provider, err := f.New(logger, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).ToNot(BeNil())
		})
	})

	Describe("IsManageable", func() {
		It("Should always return true", func() {
			manageable, prio, err := f.IsManageable(nil, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeTrue())
			Expect(prio).To(Equal(1))
		})

		It("Should return true with facts", func() {
			facts := map[string]any{
				"os": "linux",
			}
			manageable, prio, err := f.IsManageable(facts, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeTrue())
			Expect(prio).To(Equal(1))
		})
	})
})
