// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnf

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model/modelmocks"
)

func TestPackageResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Package/DNF")
}

var _ = Describe("DNF Provider", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mockctl  *gomock.Controller
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		err      error
		provider *Provider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		provider, err = NewDnfProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Status", func() {
		It("Should parse rpm output correctly for existing packages", func() {
			runner.EXPECT().Execute(gomock.Any(), "rpm", "-q", "zsh", "--queryformat", dnfNevraQueryFormat).Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/rpm_q.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			res, err := provider.Status(context.Background(), "zsh")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Metadata.Name).To(Equal("zsh"))
			Expect(res.Metadata.Version).To(Equal("5.8-9.el9"))
			Expect(res.Metadata.Arch).To(Equal("x86_64"))
			Expect(res.Metadata.Provider).To(Equal("dnf"))
			Expect(res.Metadata.Extended).To(HaveKeyWithValue("epoch", "0"))
			Expect(res.Metadata.Extended).To(HaveKeyWithValue("release", "9.el9"))
		})

		It("Should parse rpm output correctly for absent packages", func() {
			runner.EXPECT().Execute(gomock.Any(), "rpm", "-q", "zsh", "--queryformat", dnfNevraQueryFormat).Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/rpm_q_absent.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 1, nil
			})

			res, err := provider.Status(context.Background(), "zsh")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Metadata.Name).To(Equal("zsh"))
			Expect(res.Metadata.Version).To(Equal("absent"))
			Expect(res.Metadata.Arch).To(Equal(""))
			Expect(res.Metadata.Provider).To(Equal("dnf"))
			Expect(res.Metadata.Extended).To(BeEmpty())
		})
	})

	DescribeTable("Package operations",
		func(operation string, packageName string, version string, expectedCmd string, expectedArgs []string, fixtureFile string) {
			runner.EXPECT().Execute(gomock.Any(), expectedCmd, expectedArgs[0], expectedArgs[1], expectedArgs[2]).Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile(fixtureFile)
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			var err error
			switch operation {
			case "install":
				err = provider.Install(context.Background(), packageName, version)
			case "uninstall":
				err = provider.Uninstall(context.Background(), packageName)
			case "upgrade":
				err = provider.Upgrade(context.Background(), packageName, version)
			case "downgrade":
				err = provider.Downgrade(context.Background(), packageName, version)
			}
			Expect(err).ToNot(HaveOccurred())
		},
		Entry("install", "install", "zsh", "5.8", "dnf", []string{"install", "-y", "zsh-5.8"}, "testdata/dnf/dnf_install_zsh.txt"),
		Entry("uninstall", "uninstall", "zsh", "", "dnf", []string{"remove", "-y", "zsh"}, "testdata/dnf/dnf_remove.txt"),
		Entry("upgrade", "upgrade", "zsh", "6.2.3", "dnf", []string{"install", "-y", "zsh-6.2.3"}, "testdata/dnf/dnf_install_zsh.txt"),
		Entry("downgrade", "downgrade", "zsh", "0.0.1", "dnf", []string{"downgrade", "-y", "zsh-0.0.1"}, "testdata/dnf/rpm_q.txt"),
	)
})
