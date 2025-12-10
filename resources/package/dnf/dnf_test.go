// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnf

import (
	"context"
	"os"
	"testing"

	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestPackageResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types/Package/DNF")
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
		mgr, logger = modelmocks.NewManager(facts, data, mockctl)
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

	Describe("Install", func() {
		It("Should support installing", func() {
			runner.EXPECT().Execute(gomock.Any(), "dnf", "install", "-y", "zsh-5.8").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/dnf_install_zsh.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Install(context.Background(), "zsh", "5.8")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Uninstall", func() {
		It("Should support Uninstall", func() {
			runner.EXPECT().Execute(gomock.Any(), "dnf", "remove", "-y", "zsh").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/dnf_remove.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Uninstall(context.Background(), "zsh")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Upgrade", func() {
		It("Should support upgrading", func() {
			runner.EXPECT().Execute(gomock.Any(), "dnf", "install", "-y", "zsh-6.2.3").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/dnf_install_zsh.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Upgrade(context.Background(), "zsh", "6.2.3")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Downgrade", func() {
		It("Should support downgrades", func() {
			runner.EXPECT().Execute(gomock.Any(), "dnf", "downgrade", "-y", "zsh-0.0.1").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/rpm_q.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Downgrade(context.Background(), "zsh", "0.0.1")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Uninstall", func() {
		It("Should support uninstalling", func() {
			runner.EXPECT().Execute(gomock.Any(), "dnf", "remove", "-y", "zsh").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/dnf/dnf_remove.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Uninstall(context.Background(), "zsh")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
