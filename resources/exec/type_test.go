// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package execresource

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestExecResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Exec")
}

func intPtr(i int) *int {
	return &i
}

var _ = Describe("Exec Type", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		mockctl  *gomock.Controller
		provider *MockExecProvider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		provider = NewMockExecProvider(mockctl)

		provider.EXPECT().Name().Return("mock").AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("New", func() {
		It("Should validate properties", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ExecResourceProperties{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name"))
		})

		It("Should require ensure", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ensure"))
		})

		It("Should create exec resource with valid properties", func(ctx context.Context) {
			exec, err := New(ctx, mgr, model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Type:   model.ExecTypeName,
					Name:   "/bin/echo hello",
					Ensure: model.EnsurePresent,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(exec).ToNot(BeNil())
			Expect(exec.prop.Name).To(Equal("/bin/echo hello"))
		})

		It("Should set alias from properties", func(ctx context.Context) {
			exec, err := New(ctx, mgr, model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: model.EnsurePresent,
					Alias:  "greeting",
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(exec).ToNot(BeNil())
			Expect(exec.Base.CommonProperties.Alias).To(Equal("greeting"))

			event := exec.NewTransactionEvent()
			Expect(event.Alias).To(Equal("greeting"))
		})

		It("Should accept subscribe property", func(ctx context.Context) {
			exec, err := New(ctx, mgr, model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/bin/systemctl reload nginx",
					Ensure: model.EnsurePresent,
				},
				Subscribe: []string{"file#/etc/nginx/nginx.conf"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(exec.prop.Subscribe).To(HaveLen(1))
			Expect(exec.prop.Subscribe[0]).To(Equal("file#/etc/nginx/nginx.conf"))
		})

		It("Should reject invalid subscribe format", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: model.EnsurePresent,
				},
				Subscribe: []string{"invalid-format"},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid subscribe format"))
		})

		It("Should skip validation when SkipValidate is true", func(ctx context.Context) {
			exec, err := New(ctx, mgr, model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:         "",
					Ensure:       "",
					SkipValidate: true,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(exec).ToNot(BeNil())
		})
	})

	Describe("isDesiredState", func() {
		var exec *Type

		BeforeEach(func(ctx context.Context) {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: model.EnsurePresent,
				},
			}
			var err error
			exec, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("with Creates property", func() {
			It("Should return true when Creates is satisfied", func() {
				exec.prop.Creates = "/tmp/marker"
				status := &model.ExecState{CreatesSatisfied: true}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeTrue())
			})

			It("Should return false when Creates is not satisfied", func() {
				exec.prop.Creates = "/tmp/marker"
				status := &model.ExecState{CreatesSatisfied: false}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeFalse())
			})
		})

		Context("with RefreshOnly property", func() {
			It("Should return true when RefreshOnly and no exit code", func() {
				exec.prop.RefreshOnly = true
				status := &model.ExecState{ExitCode: nil}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeTrue())
			})

			It("Should check exit code when RefreshOnly but has exit code", func() {
				exec.prop.RefreshOnly = true
				status := &model.ExecState{ExitCode: intPtr(0)}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeTrue())

				status = &model.ExecState{ExitCode: intPtr(1)}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeFalse())
			})
		})

		Context("with exit code checks", func() {
			It("Should return true when exit code is 0 with default returns", func() {
				status := &model.ExecState{ExitCode: intPtr(0)}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeTrue())
			})

			It("Should return false when exit code is non-zero with default returns", func() {
				status := &model.ExecState{ExitCode: intPtr(1)}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeFalse())
			})

			It("Should use custom returns when specified", func() {
				exec.prop.Returns = []int{0, 1, 2}
				status := &model.ExecState{ExitCode: intPtr(1)}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeTrue())

				status = &model.ExecState{ExitCode: intPtr(3)}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeFalse())
			})
		})

		Context("without exit code", func() {
			It("Should return false when no exit code and not RefreshOnly", func() {
				exec.prop.RefreshOnly = false
				status := &model.ExecState{ExitCode: nil}
				Expect(exec.isDesiredState(exec.prop, status)).To(BeFalse())
			})
		})
	})

	Context("with a prepared provider", func() {
		var factory *modelmocks.MockProviderFactory
		var exec *Type
		var properties *model.ExecResourceProperties
		var err error

		BeforeEach(func(ctx context.Context) {
			factory = modelmocks.NewMockProviderFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.ExecTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})
			properties = &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "/bin/echo hello",
					Ensure:   model.EnsurePresent,
					Provider: "test",
				},
			}
			exec, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())

			registry.Clear()
			registry.MustRegister(factory)
		})

		Describe("Apply", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()
			})

			It("Should fail if initial status check fails", func(ctx context.Context) {
				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("status failed"))

				event, err := exec.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Errors).To(ContainElement(ContainSubstring("status failed")))
			})

			Context("when Creates is satisfied", func() {
				It("Should not execute when Creates file exists", func(ctx context.Context) {
					exec.prop.Creates = "/tmp/marker"
					initialState := &model.ExecState{CreatesSatisfied: true}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					// No Execute call expected

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})
			})

			Context("when command needs to run", func() {
				It("Should execute and succeed with exit code 0", func(ctx context.Context) {
					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}
					finalState := &model.ExecState{CreatesSatisfied: false, ExitCode: intPtr(0)}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Status.(*model.ExecState).ExitCode).ToNot(BeNil())
					Expect(*result.Status.(*model.ExecState).ExitCode).To(Equal(0))
				})

				It("Should fail when exit code is not in acceptable returns", func(ctx context.Context) {
					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}
					finalState := &model.ExecState{CreatesSatisfied: false, ExitCode: intPtr(1)}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(1, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					event, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Errors).To(ContainElement(ContainSubstring("failed to reach desired state")))
				})

				It("Should succeed with custom returns", func(ctx context.Context) {
					exec.prop.Returns = []int{0, 1}
					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}
					finalState := &model.ExecState{CreatesSatisfied: false, ExitCode: intPtr(1)}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(1, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Failed).To(BeFalse())
				})

				It("Should fail when Execute returns error", func(ctx context.Context) {
					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(-1, fmt.Errorf("execution failed"))

					event, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Errors).To(ContainElement(ContainSubstring("execution failed")))
				})
			})

			Context("with RefreshOnly", func() {
				BeforeEach(func() {
					exec.prop.RefreshOnly = true
				})

				It("Should not execute without subscribe trigger", func(ctx context.Context) {
					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					// No Execute call expected

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})
			})

			Context("with Subscribe", func() {
				BeforeEach(func() {
					exec.prop.Subscribe = []string{"file#/etc/app.conf"}
				})

				It("Should execute when subscribe resource changed", func(ctx context.Context) {
					mgr.EXPECT().ShouldRefresh("file", "/etc/app.conf").Return(true, nil)

					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}
					finalState := &model.ExecState{CreatesSatisfied: false, ExitCode: intPtr(0)}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Refreshed).To(BeTrue())
				})

				It("Should not execute when subscribe resource not changed", func(ctx context.Context) {
					mgr.EXPECT().ShouldRefresh("file", "/etc/app.conf").Return(false, nil)

					// Stable because Creates not set, RefreshOnly not set, and no ExitCode -> not stable
					// So it will execute
					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}
					finalState := &model.ExecState{CreatesSatisfied: false, ExitCode: intPtr(0)}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Refreshed).To(BeFalse())
				})

				It("Should execute via subscribe even when Creates is satisfied", func(ctx context.Context) {
					exec.prop.Creates = "/tmp/marker"
					mgr.EXPECT().ShouldRefresh("file", "/etc/app.conf").Return(true, nil)

					initialState := &model.ExecState{CreatesSatisfied: true, ExitCode: nil}
					finalState := &model.ExecState{CreatesSatisfied: true, ExitCode: intPtr(0)}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Refreshed).To(BeTrue())
				})

				It("Should fail when ShouldRefresh returns error", func(ctx context.Context) {
					mgr.EXPECT().ShouldRefresh("file", "/etc/app.conf").Return(false, fmt.Errorf("refresh check failed"))

					initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)

					event, err := exec.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Errors).To(ContainElement(ContainSubstring("refresh check failed")))
				})
			})
		})

		Describe("Apply in noop mode", func() {
			var noopMgr *modelmocks.MockManager
			var noopExec *Type
			var noopProvider *MockExecProvider

			BeforeEach(func(ctx context.Context) {
				noopMgr, _ = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner := modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)
				noopProvider = NewMockExecProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()

				noopFactory := modelmocks.NewMockProviderFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.ExecTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				noopProperties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "/bin/echo noop",
						Ensure:   model.EnsurePresent,
						Provider: "noop-test",
					},
				}
				var err error
				noopExec, err = New(ctx, noopMgr, *noopProperties)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should not execute in noop mode", func(ctx context.Context) {
				initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}

				noopProvider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
				// No Execute call expected

				result, err := noopExec.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have executed"))
			})

			It("Should not execute via subscribe in noop mode", func(ctx context.Context) {
				noopExec.prop.Subscribe = []string{"file#/etc/app.conf"}
				noopMgr.EXPECT().ShouldRefresh("file", "/etc/app.conf").Return(true, nil)

				initialState := &model.ExecState{CreatesSatisfied: false, ExitCode: nil}

				noopProvider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
				// No Execute call expected

				result, err := noopExec.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have executed via subscribe"))
				Expect(result.Refreshed).To(BeTrue())
			})

			It("Should not change when already in desired state", func(ctx context.Context) {
				noopExec.prop.Creates = "/tmp/marker"
				initialState := &model.ExecState{CreatesSatisfied: true, ExitCode: nil}

				noopProvider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)

				result, err := noopExec.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeFalse())
				Expect(result.Noop).To(BeTrue())
			})
		})

		Describe("Info", func() {
			It("Should return not implemented error", func(ctx context.Context) {
				_, err := exec.Info(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not implemented"))
			})
		})

		Describe("Accessor methods", func() {
			It("Should return empty provider before selection", func() {
				Expect(exec.Provider()).To(BeEmpty())
			})

			It("Should return provider name after selection", func(ctx context.Context) {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil)

				name, err := exec.SelectProvider()
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal("mock"))
				Expect(exec.Provider()).To(Equal("mock"))
			})
		})
	})
})
