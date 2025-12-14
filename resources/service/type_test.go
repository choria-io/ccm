// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package serviceresource

import (
	"context"
	"fmt"
	"testing"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestServiceResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types/Service")
}

// Helper function to create bool pointers for table tests
func boolPtr(b bool) *bool {
	return &b
}

var _ = Describe("Service Type", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		mockctl  *gomock.Controller
		provider *MockServiceProvider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		provider = NewMockServiceProvider(mockctl)

		provider.EXPECT().Name().Return("mock").AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	Describe("isDesiredState", func() {
		var svc *Type

		BeforeEach(func(ctx context.Context) {
			properties := &model.ServiceResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "test-service",
					Ensure: model.ServiceEnsureRunning,
				},
			}
			var err error
			svc, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("ensure state matching",
			func(propsEnsure, stateEnsure string, expected bool) {
				props := &model.ServiceResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{Ensure: propsEnsure},
				}
				state := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Ensure: stateEnsure},
				}
				Expect(svc.isDesiredState(props, state)).To(Equal(expected))
			},
			Entry("running matches running", model.ServiceEnsureRunning, model.ServiceEnsureRunning, true),
			Entry("running does not match stopped", model.ServiceEnsureRunning, model.ServiceEnsureStopped, false),
			Entry("stopped matches stopped", model.ServiceEnsureStopped, model.ServiceEnsureStopped, true),
			Entry("stopped does not match running", model.ServiceEnsureStopped, model.ServiceEnsureRunning, false),
		)

		DescribeTable("enable flag matching",
			func(enablePtr *bool, stateEnabled bool, expected bool) {
				props := &model.ServiceResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{Ensure: model.ServiceEnsureRunning},
					Enable:                   enablePtr,
				}
				state := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Ensure: model.ServiceEnsureRunning},
					Metadata:            &model.ServiceMetadata{Enabled: stateEnabled},
				}
				Expect(svc.isDesiredState(props, state)).To(Equal(expected))
			},
			Entry("enable true matches enabled", boolPtr(true), true, true),
			Entry("enable true does not match disabled", boolPtr(true), false, false),
			Entry("enable false matches disabled", boolPtr(false), false, true),
			Entry("enable false does not match enabled", boolPtr(false), true, false),
			Entry("enable nil ignores enabled state (enabled)", nil, true, true),
			Entry("enable nil ignores enabled state (disabled)", nil, false, true),
		)
	})

	Describe("New", func() {
		It("Should validate properties", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ServiceResourceProperties{})
			Expect(err).To(MatchError(model.ErrResourceNameRequired))
		})

		It("Should parse subscribe property correctly", func(ctx context.Context) {
			properties := model.ServiceResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "test-service",
					Ensure: model.ServiceEnsureRunning,
				},
				Subscribe: "package#nginx",
			}
			svc, err := New(ctx, mgr, properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(svc.subscribeType).To(Equal("package"))
			Expect(svc.subscribeName).To(Equal("nginx"))
		})

		It("Should fail with invalid subscribe property", func(ctx context.Context) {
			properties := model.ServiceResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "test-service",
					Ensure: model.ServiceEnsureRunning,
				},
				Subscribe: "invalid",
			}
			_, err := New(ctx, mgr, properties)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid subscribe property"))
		})
	})

	Context("with a prepared provider", func() {
		var factory *modelmocks.MockProviderFactory
		var svc *Type
		var properties *model.ServiceResourceProperties
		var err error

		BeforeEach(func(ctx context.Context) {
			factory = modelmocks.NewMockProviderFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.ServiceTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})
			properties = &model.ServiceResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "nginx",
					Ensure:   model.ServiceEnsureRunning,
					Provider: "test",
				},
			}
			svc, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())

			registry.Clear()
			registry.MustRegister(factory)
		})

		Describe("Apply", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil).AnyTimes()
			})

			It("Should fail if initial status check fails", func(ctx context.Context) {
				provider.EXPECT().Status(gomock.Any(), "nginx").Return(nil, fmt.Errorf("status failed"))

				event, err := svc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("status failed"))
			})

			Context("when ensure is running", func() {
				BeforeEach(func() {
					svc.prop.Ensure = model.ServiceEnsureRunning
				})

				It("Should start when service is stopped", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
					}
					finalState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Start(gomock.Any(), "nginx").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(finalState, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal(model.ServiceEnsureRunning))
					Expect(result.ActualEnsure).To(Equal(model.ServiceEnsureRunning))
				})

				It("Should not change when service is already running", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.ActualEnsure).To(Equal(model.ServiceEnsureRunning))
				})

				It("Should fail if start fails", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Start(gomock.Any(), "nginx").Return(fmt.Errorf("start failed"))

					event, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("start failed"))
				})
			})

			Context("when ensure is stopped", func() {
				BeforeEach(func() {
					svc.prop.Ensure = model.ServiceEnsureStopped
				})

				It("Should stop when service is running", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}
					finalState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Stop(gomock.Any(), "nginx").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(finalState, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal(model.ServiceEnsureStopped))
				})

				It("Should not change when service is already stopped", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should fail if stop fails", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Stop(gomock.Any(), "nginx").Return(fmt.Errorf("stop failed"))

					event, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("stop failed"))
				})
			})

			Context("when enable is true", func() {
				BeforeEach(func() {
					enable := true
					svc.prop.Enable = &enable
				})

				It("Should enable when service is disabled", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: false},
					}
					finalState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Enable(gomock.Any(), "nginx").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(finalState, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should not change when service is already enabled", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should fail if enable fails", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: false},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Enable(gomock.Any(), "nginx").Return(fmt.Errorf("enable failed"))

					event, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("enable failed"))
				})
			})

			Context("when enable is false", func() {
				BeforeEach(func() {
					enable := false
					svc.prop.Enable = &enable
				})

				It("Should disable when service is enabled", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: true},
					}
					finalState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: false},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Disable(gomock.Any(), "nginx").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(finalState, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should not change when service is already disabled", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: false},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should fail if disable fails", func(ctx context.Context) {
					initialState := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
					provider.EXPECT().Disable(gomock.Any(), "nginx").Return(fmt.Errorf("disable failed"))

					event, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("disable failed"))
				})
			})

			Context("when subscribe is set", func() {
				BeforeEach(func(ctx context.Context) {
					properties.Subscribe = "package#nginx"
					svc, err = New(ctx, mgr, *properties)
					Expect(err).ToNot(HaveOccurred())
				})

				It("Should restart when subscription triggers", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					mgr.EXPECT().ShouldRefresh("package", "nginx").Return(true, nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					provider.EXPECT().Restart(gomock.Any(), "nginx").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should not restart when subscription does not trigger", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					mgr.EXPECT().ShouldRefresh("package", "nginx").Return(false, nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should fail if restart fails", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					mgr.EXPECT().ShouldRefresh("package", "nginx").Return(true, nil)
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					provider.EXPECT().Restart(gomock.Any(), "nginx").Return(fmt.Errorf("restart failed"))

					event, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("restart failed"))
				})

				It("Should fail if ShouldRefresh fails", func(ctx context.Context) {
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}
					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

					mgr.EXPECT().ShouldRefresh("package", "nginx").Return(false, fmt.Errorf("refresh check failed"))

					event, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("refresh check failed"))
				})
			})

			It("Should fail if final status check fails", func(ctx context.Context) {
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
				}

				provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				provider.EXPECT().Start(gomock.Any(), "nginx").Return(nil)
				provider.EXPECT().Status(gomock.Any(), "nginx").Return(nil, fmt.Errorf("final status failed"))

				event, err := svc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("final status failed"))
			})

			It("Should fail if desired state is not reached", func(ctx context.Context) {
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
				}
				finalState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
				}

				provider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				provider.EXPECT().Start(gomock.Any(), "nginx").Return(nil)
				provider.EXPECT().Status(gomock.Any(), "nginx").Return(finalState, nil)

				event, err := svc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("failed to reach desired state"))
			})

			Context("with health check", func() {
				It("Should succeed when health check passes", func(ctx context.Context) {
					svc.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_http -H localhost",
					}
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_http", "-H", "localhost").
						Return([]byte("HTTP OK"), []byte{}, 0, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeFalse())
					Expect(result.Error).To(BeEmpty())
				})

				It("Should fail when health check returns warning", func(ctx context.Context) {
					svc.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_http -H localhost",
					}
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_http", "-H", "localhost").
						Return([]byte("HTTP WARNING - slow response"), []byte{}, 1, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(Equal("health check status \"WARNING\""))
					Expect(result.HealthCheck).ToNot(BeNil())
					Expect(result.HealthCheck.Status).To(Equal(model.HealthCheckWarning))
				})

				It("Should fail when health check returns critical", func(ctx context.Context) {
					svc.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_http -H localhost",
					}
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_http", "-H", "localhost").
						Return([]byte("HTTP CRITICAL - connection refused"), []byte{}, 2, nil)

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(Equal("health check status \"CRITICAL\""))
					Expect(result.HealthCheck).ToNot(BeNil())
					Expect(result.HealthCheck.Status).To(Equal(model.HealthCheckCritical))
				})

				It("Should fail when health check command execution fails", func(ctx context.Context) {
					svc.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_http -H localhost",
					}
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_http", "-H", "localhost").
						Return(nil, nil, 0, fmt.Errorf("command not found"))

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(ContainSubstring("command not found"))
				})

				It("Should not run health check when not configured", func(ctx context.Context) {
					svc.prop.HealthCheck = nil
					state := &model.ServiceState{
						CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
						Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
					}

					provider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)
					// No runner.EXPECT() for health check command

					result, err := svc.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeFalse())
					Expect(result.HealthCheck).To(BeNil())
				})
			})
		})

		Describe("Apply in noop mode", func() {
			var noopMgr *modelmocks.MockManager
			var noopSvc *Type
			var noopProvider *MockServiceProvider

			BeforeEach(func(ctx context.Context) {
				noopMgr, _ = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner := modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)
				noopMgr.EXPECT().ShouldRefresh(gomock.Any(), gomock.Any()).AnyTimes().Return(false, nil)
				noopProvider = NewMockServiceProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()

				noopFactory := modelmocks.NewMockProviderFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.ServiceTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts).Return(true, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				noopProperties := &model.ServiceResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "nginx",
						Ensure:   model.ServiceEnsureRunning,
						Provider: "noop-test",
					},
				}
				var err error
				noopSvc, err = New(ctx, noopMgr, *noopProperties)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should not start when service is stopped", func(ctx context.Context) {
				noopSvc.prop.Ensure = model.ServiceEnsureRunning
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				// No Start call expected

				result, err := noopSvc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have started"))
			})

			It("Should not stop when service is running", func(ctx context.Context) {
				noopSvc.prop.Ensure = model.ServiceEnsureStopped
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				// No Stop call expected

				result, err := noopSvc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have stopped"))
			})

			It("Should not enable when service is disabled", func(ctx context.Context) {
				enable := true
				noopSvc.prop.Enable = &enable
				noopSvc.prop.Ensure = model.ServiceEnsureRunning
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: false},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				// No Enable call expected

				result, err := noopSvc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have enabled"))
			})

			It("Should not disable when service is enabled", func(ctx context.Context) {
				disable := false
				noopSvc.prop.Enable = &disable
				noopSvc.prop.Ensure = model.ServiceEnsureRunning
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true, Enabled: true},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				// No Disable call expected

				result, err := noopSvc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have disabled"))
			})

			It("Should combine start and enable messages", func(ctx context.Context) {
				enable := true
				noopSvc.prop.Enable = &enable
				noopSvc.prop.Ensure = model.ServiceEnsureRunning
				initialState := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureStopped},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: false, Enabled: false},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "nginx").Return(initialState, nil)
				// No Start or Enable calls expected

				result, err := noopSvc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have started, would have enabled"))
			})

			It("Should not change when already in desired state", func(ctx context.Context) {
				noopSvc.prop.Ensure = model.ServiceEnsureRunning
				state := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx", Ensure: model.ServiceEnsureRunning},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "nginx").Return(state, nil)

				result, err := noopSvc.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeFalse())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(BeEmpty())
			})
		})

		Describe("Info", func() {
			It("Should fail if no suitable factory", func() {
				factory.EXPECT().IsManageable(facts).Return(false, nil)

				_, err := svc.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotManageable))
			})

			It("Should fail for unknown factory", func() {
				svc.prop.Provider = "unknown"
				_, err := svc.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotFound))
			})

			It("Should handle info failures", func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil)
				provider.EXPECT().Status(gomock.Any(), "nginx").Return(nil, fmt.Errorf("cant execute status command"))

				nfo, err := svc.Info(context.Background())
				Expect(err).To(Equal(fmt.Errorf("cant execute status command")))
				Expect(nfo).To(BeNil())
			})

			It("Should call status on the provider", func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil)

				res := &model.ServiceState{
					CommonResourceState: model.CommonResourceState{Name: "nginx"},
					Metadata:            &model.ServiceMetadata{Name: "nginx", Running: true},
				}
				provider.EXPECT().Status(gomock.Any(), "nginx").Return(res, nil)

				nfo, err := svc.Info(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(nfo).ToNot(BeNil())
				Expect(nfo).To(Equal(res))
			})
		})
	})
})
