// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestServiceResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types/Service/Systemd")
}

var _ = Describe("Systemd Provider", func() {
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

		provider, err = NewSystemdProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("systemd"))
		})
	})

	Describe("isEnabled", func() {
		DescribeTable("systemctl is-enabled output parsing",
			func(fixtureFile, serviceName string, expectedEnabled bool, expectError bool) {
				runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", serviceName).Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
					if fixtureFile != "" {
						stdout, err := os.ReadFile(fixtureFile)
						Expect(err).ToNot(HaveOccurred())
						return stdout, nil, 0, nil
					}
					return []byte("unknown-state\n"), nil, 0, nil
				})

				enabled, err := provider.isEnabled(context.Background(), serviceName)
				if expectError {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid systemctl is-enabled output"))
					Expect(enabled).To(BeFalse())
				} else {
					Expect(err).ToNot(HaveOccurred())
					Expect(enabled).To(Equal(expectedEnabled))
				}
			},
			Entry("enabled", "testdata/systemd/is-enabled-enabled.txt", "nginx", true, false),
			Entry("enabled-runtime", "testdata/systemd/is-enabled-enabled-runtime.txt", "nginx", true, false),
			Entry("alias", "testdata/systemd/is-enabled-alias.txt", "nginx", true, false),
			Entry("static", "testdata/systemd/is-enabled-static.txt", "dbus", true, false),
			Entry("indirect", "testdata/systemd/is-enabled-indirect.txt", "nginx", true, false),
			Entry("generated", "testdata/systemd/is-enabled-generated.txt", "nginx", true, false),
			Entry("transient", "testdata/systemd/is-enabled-transient.txt", "nginx", true, false),
			Entry("disabled", "testdata/systemd/is-enabled-disabled.txt", "nginx", false, false),
			Entry("linked", "testdata/systemd/is-enabled-linked.txt", "nginx", false, false),
			Entry("linked-runtime", "testdata/systemd/is-enabled-linked-runtime.txt", "nginx", false, false),
			Entry("masked", "testdata/systemd/is-enabled-masked.txt", "nginx", false, false),
			Entry("masked-runtime", "testdata/systemd/is-enabled-masked-runtime.txt", "nginx", false, false),
			Entry("invalid output", "", "nginx", false, true),
		)
	})

	Describe("isActive", func() {
		DescribeTable("systemctl is-active output parsing",
			func(fixtureFile string, expectedActive bool, expectError bool) {
				runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
					if fixtureFile != "" {
						stdout, err := os.ReadFile(fixtureFile)
						Expect(err).ToNot(HaveOccurred())
						return stdout, nil, 0, nil
					}
					return []byte("unknown-state\n"), nil, 0, nil
				})

				active, err := provider.isActive(context.Background(), "nginx")
				if expectError {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid systemctl is-active output"))
					Expect(active).To(BeFalse())
				} else {
					Expect(err).ToNot(HaveOccurred())
					Expect(active).To(Equal(expectedActive))
				}
			},
			Entry("active", "testdata/systemd/is-active-active.txt", true, false),
			Entry("inactive", "testdata/systemd/is-active-inactive.txt", false, false),
			Entry("failed", "testdata/systemd/is-active-failed.txt", false, false),
			Entry("activating", "testdata/systemd/is-active-activating.txt", false, false),
			Entry("invalid output", "", false, true),
		)
	})

	Describe("Status", func() {
		It("Should report running and enabled service correctly", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-active.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-enabled.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Name).To(Equal("nginx"))
			Expect(status.Ensure).To(Equal(model.ServiceEnsureRunning))
			Expect(status.Metadata).ToNot(BeNil())
			Expect(status.Metadata.Name).To(Equal("nginx"))
			Expect(status.Metadata.Provider).To(Equal("systemd"))
			Expect(status.Metadata.Running).To(BeTrue())
			Expect(status.Metadata.Enabled).To(BeTrue())
		})

		It("Should report stopped and disabled service correctly", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-inactive.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-disabled.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Name).To(Equal("nginx"))
			Expect(status.Ensure).To(Equal(model.ServiceEnsureStopped))
			Expect(status.Metadata).ToNot(BeNil())
			Expect(status.Metadata.Name).To(Equal("nginx"))
			Expect(status.Metadata.Provider).To(Equal("systemd"))
			Expect(status.Metadata.Running).To(BeFalse())
			Expect(status.Metadata.Enabled).To(BeFalse())
		})

		It("Should report running but disabled service correctly", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-active.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-disabled.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.ServiceEnsureRunning))
			Expect(status.Metadata.Running).To(BeTrue())
			Expect(status.Metadata.Enabled).To(BeFalse())
		})

		It("Should report stopped but enabled service correctly", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-inactive.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-enabled.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.ServiceEnsureStopped))
			Expect(status.Metadata.Running).To(BeFalse())
			Expect(status.Metadata.Enabled).To(BeTrue())
		})

		It("Should report failed service as stopped", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-failed.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-enabled.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.ServiceEnsureStopped))
			Expect(status.Metadata.Running).To(BeFalse())
			Expect(status.Metadata.Enabled).To(BeTrue())
		})

		It("Should report masked service correctly", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-inactive.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-masked.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.ServiceEnsureStopped))
			Expect(status.Metadata.Running).To(BeFalse())
			Expect(status.Metadata.Enabled).To(BeFalse())
		})

		It("Should report static service as enabled when running", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-active", "--system", "dbus").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-active-active.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			runner.EXPECT().Execute(gomock.Any(), "systemctl", "is-enabled", "--system", "dbus").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				stdout, err := os.ReadFile("testdata/systemd/is-enabled-static.txt")
				Expect(err).ToNot(HaveOccurred())
				return stdout, nil, 0, nil
			})

			status, err := provider.Status(context.Background(), "dbus")
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.ServiceEnsureRunning))
			Expect(status.Metadata.Running).To(BeTrue())
			Expect(status.Metadata.Enabled).To(BeTrue())
		})
	})

	Describe("Enable", func() {
		It("Should call systemctl enable", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "enable", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return []byte(""), nil, 0, nil
			})

			err := provider.Enable(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should propagate errors from systemctl", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "enable", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return nil, []byte("Failed to enable unit"), 1, fmt.Errorf("execution failed")
			})

			err := provider.Enable(context.Background(), "nginx")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Disable", func() {
		It("Should call systemctl disable", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "disable", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return []byte(""), nil, 0, nil
			})

			err := provider.Disable(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should propagate errors from systemctl", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "disable", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return nil, []byte("Failed to disable unit"), 1, fmt.Errorf("execution failed")
			})

			err := provider.Disable(context.Background(), "nginx")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Start", func() {
		It("Should call systemctl start", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "start", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return []byte(""), nil, 0, nil
			})

			err := provider.Start(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should propagate errors from systemctl", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "start", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return nil, []byte("Failed to start unit"), 1, fmt.Errorf("execution failed")
			})

			err := provider.Start(context.Background(), "nginx")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Stop", func() {
		It("Should call systemctl stop", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "stop", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return []byte(""), nil, 0, nil
			})

			err := provider.Stop(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should propagate errors from systemctl", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "stop", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return nil, []byte("Failed to stop unit"), 1, fmt.Errorf("execution failed")
			})

			err := provider.Stop(context.Background(), "nginx")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Restart", func() {
		It("Should call systemctl restart", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "restart", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return []byte(""), nil, 0, nil
			})

			err := provider.Restart(context.Background(), "nginx")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should propagate errors from systemctl", func() {
			runner.EXPECT().Execute(gomock.Any(), "systemctl", "restart", "--system", "nginx").Times(1).DoAndReturn(func(ctx context.Context, cmd string, args ...string) ([]byte, []byte, int, error) {
				return nil, []byte("Failed to restart unit"), 1, fmt.Errorf("execution failed")
			})

			err := provider.Restart(context.Background(), "nginx")
			Expect(err).To(HaveOccurred())
		})
	})
})
