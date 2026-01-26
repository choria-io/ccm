// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestShell(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Exec/Shell")
}

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
		var originalShellPath string

		BeforeEach(func() {
			originalShellPath = shellPath
		})

		AfterEach(func() {
			shellPath = originalShellPath
		})

		It("Should return true when shell exists", func() {
			shellPath = "/bin/sh"
			manageable, prio, err := f.IsManageable(nil, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeTrue())
			Expect(prio).To(Equal(99))
		})

		It("Should return true with facts when shell exists", func() {
			shellPath = "/bin/sh"
			facts := map[string]any{
				"os": "linux",
			}
			manageable, _, err := f.IsManageable(facts, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeTrue())
		})

		It("Should return false when shell does not exist", func() {
			shellPath = "/nonexistent/shell/path"
			manageable, _, err := f.IsManageable(nil, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeFalse())
		})
	})
})
