// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apt

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestAptProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types/Package/APT")
}

var _ = Describe("APT Provider", func() {
	var (
		mockctl  *gomock.Controller
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		provider *Provider
		err      error
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)

		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		provider, err = NewAptProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("Name", func() {
		It("Should return apt", func() {
			Expect(provider.Name()).To(Equal("apt"))
		})
	})

	Describe("Status", func() {
		It("Should parse dpkg-query output correctly for installed packages", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("dpkg-query"))
				Expect(opts.Args).To(Equal([]string{"-W", "-f=${Package} ${Version} ${Architecture} ${db:Status-Status}", "zsh"}))
				stdout, err := os.ReadFile("testdata/dpkg_query_installed.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			res, err := provider.Status(context.Background(), "zsh")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Metadata.Name).To(Equal("zsh"))
			Expect(res.Metadata.Version).To(Equal("5.9-8+b18"))
			Expect(res.Metadata.Arch).To(Equal("amd64"))
			Expect(res.Metadata.Provider).To(Equal("apt"))
		})

		It("Should parse dpkg-query output correctly for packages with epoch", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("dpkg-query"))
				stdout, err := os.ReadFile("testdata/dpkg_query_epoch.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			res, err := provider.Status(context.Background(), "vim")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Metadata.Name).To(Equal("vim"))
			Expect(res.Metadata.Version).To(Equal("2:9.1.1230-2"))
			Expect(res.Metadata.Arch).To(Equal("amd64"))
			Expect(res.Metadata.Provider).To(Equal("apt"))
		})

		It("Should return absent status for missing packages", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("dpkg-query"))
				stderr, err := os.ReadFile("testdata/dpkg_query_absent.txt")
				Expect(err).ToNot(HaveOccurred())
				return nil, stderr, 1, nil
			})

			res, err := provider.Status(context.Background(), "nonexistent-pkg")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Metadata.Name).To(Equal("nonexistent-pkg"))
			Expect(res.Metadata.Version).To(Equal("absent"))
			Expect(res.Metadata.Provider).To(Equal("apt"))
		})
	})

	Describe("Install", func() {
		It("Should install a package with ensure present", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				Expect(opts.Args).To(Equal([]string{"install", "-y", "-q", "-o", "DPkg::Options::=--force-confold", "zsh"}))
				Expect(opts.Environment).To(ContainElement("DEBIAN_FRONTEND=noninteractive"))
				stdout, err := os.ReadFile("testdata/apt_get_install.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Install(context.Background(), "zsh", model.EnsurePresent)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should install a package with specific version", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				Expect(opts.Args).To(Equal([]string{"install", "-y", "-q", "-o", "DPkg::Options::=--force-confold", "--allow-downgrades", "zsh=5.9-8+b18"}))
				stdout, err := os.ReadFile("testdata/apt_get_install.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Install(context.Background(), "zsh", "5.9-8+b18")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should install a package with ensure latest", func() {
			// First call to apt-cache policy for latestAvailable
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-cache"))
				Expect(opts.Args).To(Equal([]string{"policy", "zsh"}))
				stdout, err := os.ReadFile("testdata/apt_cache_policy.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			// Second call to apt-get install
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				Expect(opts.Args).To(Equal([]string{"install", "-y", "-q", "-o", "DPkg::Options::=--force-confold", "zsh=5.9-8+b18"}))
				stdout, err := os.ReadFile("testdata/apt_get_install.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Install(context.Background(), "zsh", model.PackageEnsureLatest)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should return error when package not found", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				stdout, err := os.ReadFile("testdata/apt_get_install_fail_stdout.txt")
				Expect(err).ToNot(HaveOccurred())
				stderr, err := os.ReadFile("testdata/apt_get_install_fail_stderr.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, stderr, 100, nil
			})

			err := provider.Install(context.Background(), "nonexistent-pkg", model.EnsurePresent)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to install package"))
			Expect(err.Error()).To(ContainSubstring("apt-get exited 100"))
		})
	})

	Describe("Uninstall", func() {
		It("Should uninstall a package successfully", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				Expect(opts.Args).To(Equal([]string{"-q", "-y", "remove", "zsh"}))
				stdout, err := os.ReadFile("testdata/apt_get_remove.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Uninstall(context.Background(), "zsh")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should return error with stderr when uninstall fails with exit code 100", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				stdout, err := os.ReadFile("testdata/apt_get_remove_fail_stdout.txt")
				Expect(err).ToNot(HaveOccurred())
				stderr, err := os.ReadFile("testdata/apt_get_remove_fail_stderr.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, stderr, 100, nil
			})

			err := provider.Uninstall(context.Background(), "nonexistent-pkg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to uninstall nonexistent-pkg"))
			Expect(err.Error()).To(ContainSubstring("Unable to locate package"))
		})
	})

	Describe("Upgrade", func() {
		It("Should delegate to Install", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				Expect(opts.Args).To(ContainElement("zsh=6.0"))
				stdout, err := os.ReadFile("testdata/apt_get_install.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Upgrade(context.Background(), "zsh", "6.0")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Downgrade", func() {
		It("Should delegate to Install", func() {
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
				Expect(opts.Command).To(Equal("apt-get"))
				Expect(opts.Args).To(ContainElement("zsh=5.8"))
				Expect(opts.Args).To(ContainElement("--allow-downgrades"))
				stdout, err := os.ReadFile("testdata/apt_get_install.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			err := provider.Downgrade(context.Background(), "zsh", "5.8")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("VersionCmp", func() {
		It("Should compare versions correctly", func() {
			cmp, err := provider.VersionCmp("1.0", "2.0", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(-1))

			cmp, err = provider.VersionCmp("2.0", "1.0", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(1))

			cmp, err = provider.VersionCmp("1.0", "1.0", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(0))
		})

		It("Should handle versions with epochs", func() {
			cmp, err := provider.VersionCmp("1:1.0", "2.0", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(1)) // epoch 1 > no epoch
		})
	})

	Describe("parseLatestAvailable", func() {
		It("Should parse candidate version from apt-cache policy output", func() {
			output := `zsh:
  Installed: (none)
  Candidate: 5.9-8+b18
  Version table:
     5.9-8+b18 500
        500 http://deb.debian.org/debian trixie/main amd64 Packages`

			version, err := parseLatestAvailable(output, "zsh")
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(Equal("5.9-8+b18"))
		})

		It("Should parse candidate version when package is installed", func() {
			output := `vim:
  Installed: 2:9.0.1378-2
  Candidate: 2:9.0.1378-2
  Version table:
 *** 2:9.0.1378-2 500
        500 http://deb.debian.org/debian bookworm/main amd64 Packages
        100 /var/lib/dpkg/status`

			version, err := parseLatestAvailable(output, "vim")
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(Equal("2:9.0.1378-2"))
		})

		It("Should handle version with complex suffix", func() {
			output := `nginx:
  Installed: (none)
  Candidate: 1.22.1-9+deb12u1
  Version table:
     1.22.1-9+deb12u1 500
        500 http://deb.debian.org/debian bookworm/main amd64 Packages`

			version, err := parseLatestAvailable(output, "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(Equal("1.22.1-9+deb12u1"))
		})

		It("Should handle (none) candidate for unavailable packages", func() {
			output := `nonexistent:
  Installed: (none)
  Candidate: (none)
  Version table:`

			version, err := parseLatestAvailable(output, "nonexistent")
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(Equal("(none)"))
		})

		It("Should return error when Candidate line is missing", func() {
			output := `invalid output without candidate line`

			_, err := parseLatestAvailable(output, "pkg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find Candidate:"))
		})

		It("Should return error for empty output", func() {
			_, err := parseLatestAvailable("", "pkg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find Candidate:"))
		})

		It("Should handle candidate line with extra whitespace", func() {
			output := `pkg:
  Installed: (none)
    Candidate:    1.0.0
  Version table:`

			version, err := parseLatestAvailable(output, "pkg")
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("Should parse from real fixture file", func() {
			output, err := os.ReadFile("testdata/apt_cache_policy.txt")
			Expect(err).ToNot(HaveOccurred())

			version, err := parseLatestAvailable(string(output), "zsh")
			Expect(err).ToNot(HaveOccurred())
			Expect(version).To(Equal("5.9-8+b18"))
		})
	})
})
