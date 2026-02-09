// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package goss

import (
	"context"
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestGossHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GossHealthCheck")
}

var _ = Describe("Execute", func() {
	var (
		mgr     *modelmocks.MockManager
		logger  *modelmocks.MockLogger
		mockctl *gomock.Controller
		facts   = make(map[string]any)
		data    = make(map[string]any)
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	})

	It("should return error when goss_check is empty", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{}

		result, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).To(MatchError(ErrRulesNotSpecified))
		Expect(result).To(BeNil())
	})

	It("should return OK for a passing check", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{
			GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0`),
			Format: model.HealthCheckGossFormat,
		}

		result, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Status).To(Equal(model.HealthCheckOK))
		Expect(result.Tries).To(Equal(1))
		Expect(result.Output).ToNot(BeEmpty())
	})

	It("should return CRITICAL for a failing check", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{
			GossRules: yaml.RawMessage(`command:
  "false":
    exit-status: 0`),
			Format: model.HealthCheckGossFormat,
		}

		result, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(BeNil())
		Expect(result.Status).To(Equal(model.HealthCheckCritical))
		Expect(result.Tries).To(Equal(1))
	})

	It("should populate output with summary line", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{
			GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0`),
			Format: model.HealthCheckGossFormat,
		}

		result, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Output).To(ContainSubstring("Count:"))
	})

	It("should handle nil userLogger", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{
			GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0`),
			Format: model.HealthCheckGossFormat,
		}

		result, err := Execute(ctx, mgr, hc, nil, logger)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Status).To(Equal(model.HealthCheckOK))
	})

	It("should return error for invalid goss spec", func(ctx context.Context) {
		hc := &model.CommonHealthCheck{
			GossRules: yaml.RawMessage(`not: valid: goss: {{{yaml`),
			Format:    model.HealthCheckGossFormat,
		}

		_, err := Execute(ctx, mgr, hc, logger, logger)

		Expect(err).To(HaveOccurred())
	})

	Context("with retries", func() {
		It("should default Tries=0 to 1 attempt", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "false":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
				Tries:  0,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Tries).To(Equal(1))
			Expect(result.Status).To(Equal(model.HealthCheckCritical))
		})

		It("should retry up to configured tries", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "false":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
				Tries:  3,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Tries).To(Equal(3))
			Expect(result.Status).To(Equal(model.HealthCheckCritical))
		})

		It("should stop retrying on success", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
				Tries:  3,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Tries).To(Equal(1))
			Expect(result.Status).To(Equal(model.HealthCheckOK))
		})

		It("should respect context cancellation before first attempt", func(ctx context.Context) {
			ctx, cancel := context.WithCancel(ctx)
			cancel()

			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
				Tries:  3,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).To(MatchError(context.Canceled))
			Expect(result).To(BeNil())
		})
	})

	Context("with template resolution", func() {
		It("should resolve template expressions from facts", func(ctx context.Context) {
			facts["expected_cmd"] = "true"

			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "{{ lookup("facts.expected_cmd", "") }}":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckOK))
		})

		It("should resolve template expressions from data", func(ctx context.Context) {
			data["exit_code"] = 0

			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: {{ Data.exit_code }}`),
				Format: model.HealthCheckGossFormat,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckOK))
		})

		It("should return error for invalid template expressions", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "{{ invalid_expression( }}":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
			}

			_, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).To(HaveOccurred())
		})
	})

	Context("with multiple checks", func() {
		It("should return OK when all checks pass", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0
  "echo hello":
    exit-status: 0
    stdout:
      - hello`),
				Format: model.HealthCheckGossFormat,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckOK))
		})

		It("should return CRITICAL when any check fails", func(ctx context.Context) {
			hc := &model.CommonHealthCheck{
				GossRules: yaml.RawMessage(`command:
  "true":
    exit-status: 0
  "false":
    exit-status: 0`),
				Format: model.HealthCheckGossFormat,
			}

			result, err := Execute(ctx, mgr, hc, logger, logger)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Status).To(Equal(model.HealthCheckCritical))
		})
	})
})
