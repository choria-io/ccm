// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HealthCheck")
}

var _ = Describe("ParseNagiosExitCode", func() {
	DescribeTable("parses exit codes correctly",
		func(exitCode int, output string, expectedStatus model.HealthCheckStatus) {
			result := ParseNagiosExitCode(exitCode, output)

			Expect(result.Status).To(Equal(expectedStatus))
			Expect(result.Output).To(Equal(output))
		},

		Entry("OK exit code", 0, "All checks passed", model.HealthCheckOK),
		Entry("WARNING exit code", 1, "Disk usage at 80%", model.HealthCheckWarning),
		Entry("CRITICAL exit code", 2, "Disk full", model.HealthCheckCritical),
		Entry("UNKNOWN exit code", 3, "Check failed to run", model.HealthCheckUnknown),
		Entry("unexpected exit code 4", 4, "Unexpected error", model.HealthCheckUnknown),
		Entry("unexpected exit code 127", 127, "Command not found", model.HealthCheckUnknown),
		Entry("negative exit code", -1, "Signal received", model.HealthCheckUnknown),
	)

	It("should trim whitespace from output", func() {
		result := ParseNagiosExitCode(0, "  OK - All good  \n")
		Expect(result.Output).To(Equal("OK - All good"))
	})

	It("should handle empty output", func() {
		result := ParseNagiosExitCode(0, "")
		Expect(result.Output).To(Equal(""))
		Expect(result.Status).To(Equal(model.HealthCheckOK))
	})
})

var _ = Describe("Execute", func() {
	var (
		mgr     *modelmocks.MockManager
		runner  *modelmocks.MockCommandRunner
		mockctl *gomock.Controller
		logger  model.Logger
		facts   = make(map[string]any)
		data    = make(map[string]any)
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
	})

	It("should return error when command is empty", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{Command: ""}

		result, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).To(MatchError(ErrCommandNotSpecified))
		Expect(result).To(BeNil())
	})

	DescribeTable("exit code handling",
		func(ctx context.Context, exitCode int, output string, expectedStatus model.HealthCheckStatus) {
			hc := &model.CommonHealthCheck{Command: "/usr/lib/nagios/plugins/check_disk"}

			runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
				Return([]byte(output), []byte{}, exitCode, nil)

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Status).To(Equal(expectedStatus))
			Expect(result.Output).To(Equal(output))
		},

		Entry("OK exit code", 0, "DISK OK - free space: / 50%", model.HealthCheckOK),
		Entry("WARNING exit code", 1, "DISK WARNING - free space: / 15%", model.HealthCheckWarning),
		Entry("CRITICAL exit code", 2, "DISK CRITICAL - free space: / 5%", model.HealthCheckCritical),
		Entry("UNKNOWN exit code", 3, "DISK UNKNOWN - unable to check", model.HealthCheckUnknown),
		Entry("unexpected exit code 127", 127, "Unexpected error", model.HealthCheckUnknown),
	)

	DescribeTable("command parsing",
		func(ctx context.Context, command string, expectedCmd string, expectedArgs []string) {
			hc := &model.CommonHealthCheck{Command: command}

			if len(expectedArgs) > 0 {
				args := make([]any, len(expectedArgs))
				for i, a := range expectedArgs {
					args[i] = a
				}
				runner.EXPECT().Execute(gomock.Any(), expectedCmd, args...).
					Return([]byte("OK"), []byte{}, 0, nil)
			} else {
				runner.EXPECT().Execute(gomock.Any(), expectedCmd).
					Return([]byte("OK"), []byte{}, 0, nil)
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckOK))
		},

		Entry("command with no arguments", "/usr/lib/nagios/plugins/check_ping", "/usr/lib/nagios/plugins/check_ping", []string{}),
		Entry("command with arguments", "/usr/lib/nagios/plugins/check_disk -w 20% -c 10%", "/usr/lib/nagios/plugins/check_disk", []string{"-w", "20%", "-c", "10%"}),
		Entry("command with quoted arguments", `/usr/lib/nagios/plugins/check_http -H "example.com"`, "/usr/lib/nagios/plugins/check_http", []string{"-H", "example.com"}),
	)

	It("should propagate runner creation errors", func(ctx context.Context) {
		mockctl2 := gomock.NewController(GinkgoT())
		mgr2, _ := modelmocks.NewManager(facts, data, false, mockctl2)
		expectedErr := fmt.Errorf("failed to create runner")
		mgr2.EXPECT().NewRunner().Return(nil, expectedErr)

		hc := &model.CommonHealthCheck{Command: "/bin/true"}

		result, err := Execute(ctx, mgr2, hc, logger, logger)

		Expect(err).To(MatchError(expectedErr))
		Expect(result).To(BeNil())
	})

	It("should propagate command execution errors", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{Command: "/usr/lib/nagios/plugins/check_disk"}
		expectedErr := fmt.Errorf("execution failed")

		runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
			Return(nil, nil, 0, expectedErr)

		result, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).To(MatchError(expectedErr))
		Expect(result).To(BeNil())
	})

	Context("with retries", func() {
		DescribeTable("tries configuration",
			func(ctx context.Context, tries int, expectedCalls int, finalExitCode int, expectedStatus model.HealthCheckStatus) {
				hc := &model.CommonHealthCheck{
					Command: "/usr/lib/nagios/plugins/check_disk",
					Tries:   tries,
				}

				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK CRITICAL"), []byte{}, finalExitCode, nil).Times(expectedCalls)

				result, err := Execute(ctx, mgr, hc, logger, logger)

				Expect(err).ToNot(HaveOccurred())
				Expect(result.Status).To(Equal(expectedStatus))
			},

			Entry("Tries=0 defaults to 1 attempt", 0, 1, 2, model.HealthCheckCritical),
			Entry("Tries=1 executes once", 1, 1, 2, model.HealthCheckCritical),
			Entry("Tries=3 with OK on first exits early", 3, 1, 0, model.HealthCheckOK),
		)

		It("should retry and succeed on second attempt", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				Command: "/usr/lib/nagios/plugins/check_disk",
				Tries:   3,
			}

			gomock.InOrder(
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK CRITICAL"), []byte{}, 2, nil),
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK OK"), []byte{}, 0, nil),
			)

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckOK))
			Expect(result.Output).To(Equal("DISK OK"))
		})

		It("should retry and succeed on third attempt", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				Command: "/usr/lib/nagios/plugins/check_disk",
				Tries:   3,
			}

			gomock.InOrder(
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK CRITICAL"), []byte{}, 2, nil),
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK WARNING"), []byte{}, 1, nil),
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK OK"), []byte{}, 0, nil),
			)

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckOK))
		})

		It("should return last result after all retries exhausted", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				Command: "/usr/lib/nagios/plugins/check_disk",
				Tries:   3,
			}

			gomock.InOrder(
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK CRITICAL - attempt 1"), []byte{}, 2, nil),
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK CRITICAL - attempt 2"), []byte{}, 2, nil),
				runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
					Return([]byte("DISK WARNING - attempt 3"), []byte{}, 1, nil),
			)

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckWarning))
			Expect(result.Output).To(Equal("DISK WARNING - attempt 3"))
		})

		It("should not retry on execution error", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				Command: "/usr/lib/nagios/plugins/check_disk",
				Tries:   3,
			}
			expectedErr := fmt.Errorf("execution failed")

			runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
				Return(nil, nil, 0, expectedErr).Times(1)

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).To(MatchError(expectedErr))
			Expect(result).To(BeNil())
		})

		It("should respect context cancellation between retries", func(ctx context.Context) {
			ctx, cancel := context.WithCancel(ctx)
			hc := &model.CommonHealthCheck{
				Command: "/usr/lib/nagios/plugins/check_disk",
				Tries:   3,
			}

			runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk").
				DoAndReturn(func(_ context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
					cancel() // Cancel context after first attempt
					return []byte("DISK CRITICAL"), []byte{}, 2, nil
				}).Times(1)

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).To(MatchError(context.Canceled))
			Expect(result).To(BeNil())
		})

		It("should respect context cancellation before first attempt", func(ctx context.Context) {
			ctx, cancel := context.WithCancel(ctx)
			cancel() // Cancel before execution

			hc := &model.CommonHealthCheck{
				Command: "/usr/lib/nagios/plugins/check_disk",
				Tries:   3,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).To(MatchError(context.Canceled))
			Expect(result).To(BeNil())
		})
	})
})
