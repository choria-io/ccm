// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package packageresource

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

func TestPackageResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types/Package")
}

var _ = Describe("Package Type", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		mockctl  *gomock.Controller
		provider *MockPackageProvider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		provider = NewMockPackageProvider(mockctl)

		provider.EXPECT().Name().Return("mock").AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	Describe("isDesiredState", func() {
		var pkg *Type

		BeforeEach(func(ctx context.Context) {
			properties := &model.PackageResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "test-package",
					Ensure: EnsurePresent,
				},
			}
			var err error
			pkg, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("ensure state matching",
			func(propsEnsure, stateEnsure string, expected bool) {
				props := &model.PackageResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{Ensure: propsEnsure},
				}
				state := &model.PackageState{
					CommonResourceState: model.CommonResourceState{Ensure: stateEnsure},
				}
				Expect(pkg.isDesiredState(props, state)).To(Equal(expected))
			},
			Entry("present matches any version", EnsurePresent, "1.2.3", true),
			Entry("present does not match absent", EnsurePresent, EnsureAbsent, false),
			Entry("absent matches absent", EnsureAbsent, EnsureAbsent, true),
			Entry("absent does not match present", EnsureAbsent, "1.2.3", false),
			Entry("latest matches any version", EnsureLatest, "1.2.3", true),
			Entry("latest does not match absent", EnsureLatest, EnsureAbsent, false),
			Entry("specific version matches same version", "1.2.3", "1.2.3", true),
			Entry("specific version does not match different version", "1.2.3", "1.2.4", false),
			Entry("specific version does not match absent", "1.2.3", EnsureAbsent, false),
		)
	})

	Describe("New", func() {
		It("Should validate properties", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.PackageResourceProperties{})
			Expect(err).To(MatchError(model.ErrResourceNameRequired))

			_, err = New(ctx, mgr, model.PackageResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{Name: "foo"},
			})
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})
	})

	Context("with a prepared provider", func() {
		var factory *modelmocks.MockProviderFactory
		var pkg *Type
		var properties *model.PackageResourceProperties
		var err error

		BeforeEach(func(ctx context.Context) {
			factory = modelmocks.NewMockProviderFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.PackageTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})
			properties = &model.PackageResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "zsh",
					Ensure:   "present",
					Provider: "test",
				},
			}
			pkg, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())

			registry.Clear()
			registry.MustRegister(factory)
		})

		Describe("Apply", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil).AnyTimes()
			})

			It("Should fail with empty ensure", func(ctx context.Context) {
				provider.EXPECT().Status(gomock.Any(), "zsh").Return(&model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}, nil)
				pkg.prop.Ensure = ""
				event, err := pkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("invalid value for ensure"))
			})

			It("Should fail if initial status check fails", func(ctx context.Context) {
				provider.EXPECT().Status(gomock.Any(), "zsh").Return(nil, fmt.Errorf("status failed"))

				event, err := pkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("status failed"))
			})

			Context("when ensure is present", func() {
				BeforeEach(func() {
					pkg.prop.Ensure = EnsurePresent
				})

				It("Should install when package is absent", func(ctx context.Context) {
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}
					finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Install(gomock.Any(), "zsh", EnsurePresent).Return(nil)
					provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal("present"))
					Expect(result.ActualEnsure).To(Equal("1.0.0"))
				})

				It("Should not change when package is already present", func(ctx context.Context) {
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.ActualEnsure).To(Equal("1.0.0"))
				})

				It("Should fail if install fails", func(ctx context.Context) {
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Install(gomock.Any(), "zsh", EnsurePresent).Return(fmt.Errorf("install failed"))

					event, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("install failed"))
				})
			})

			Context("when ensure is absent", func() {
				BeforeEach(func() {
					pkg.prop.Ensure = EnsureAbsent
				})

				It("Should uninstall when package is present", func(ctx context.Context) {
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}
					finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Uninstall(gomock.Any(), "zsh").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal(EnsureAbsent))
				})

				It("Should fail if uninstall fails", func(ctx context.Context) {
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Uninstall(gomock.Any(), "zsh").Return(fmt.Errorf("uninstall failed"))

					event, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("uninstall failed"))
				})
			})

			Context("when ensure is latest", func() {
				BeforeEach(func() {
					pkg.prop.Ensure = EnsureLatest
				})

				It("Should upgrade package", func(ctx context.Context) {
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}
					finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "2.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Upgrade(gomock.Any(), "zsh", EnsureLatest).Return(nil)
					provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Status.(model.PackageState).Ensure).To(Equal("2.0.0"))
				})

				It("Should fail if upgrade fails", func(ctx context.Context) {
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Upgrade(gomock.Any(), "zsh", EnsureLatest).Return(fmt.Errorf("upgrade failed"))

					event, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(Equal("upgrade failed"))
				})
			})

			Context("when ensure is a specific version", func() {
				It("Should install when package is absent", func(ctx context.Context) {
					pkg.prop.Ensure = "2.0.0"
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}
					finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "2.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Install(gomock.Any(), "zsh", "2.0.0").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal("2.0.0"))
					Expect(result.ActualEnsure).To(Equal("2.0.0"))
				})

				It("Should not change when version matches", func(ctx context.Context) {
					pkg.prop.Ensure = "1.0.0"
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.Ensure).To(Equal("1.0.0"))
					Expect(result.ActualEnsure).To(Equal("1.0.0"))
				})

				It("Should upgrade when current version is lower", func(ctx context.Context) {
					pkg.prop.Ensure = "2.0.0"
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}
					finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "2.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Upgrade(gomock.Any(), "zsh", "2.0.0").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal("2.0.0"))
					Expect(result.ActualEnsure).To(Equal("2.0.0"))
				})

				It("Should downgrade when current version is higher", func(ctx context.Context) {
					pkg.prop.Ensure = "1.0.0"
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "2.0.0"}}
					finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Downgrade(gomock.Any(), "zsh", "1.0.0").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal("1.0.0"))
					Expect(result.ActualEnsure).To(Equal("1.0.0"))
				})

				It("Should fail if upgrade fails", func(ctx context.Context) {
					pkg.prop.Ensure = "2.0.0"
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Upgrade(gomock.Any(), "zsh", "2.0.0").Return(fmt.Errorf("upgrade failed"))

					event, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("upgrade failed"))
				})

				It("Should fail if downgrade fails", func(ctx context.Context) {
					pkg.prop.Ensure = "1.0.0"
					initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "2.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
					provider.EXPECT().Downgrade(gomock.Any(), "zsh", "1.0.0").Return(fmt.Errorf("downgrade failed"))

					event, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("downgrade failed"))
				})
			})

			It("Should fail if final status check fails", func(ctx context.Context) {
				pkg.prop.Ensure = EnsureLatest
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

				provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				provider.EXPECT().Upgrade(gomock.Any(), "zsh", EnsureLatest).Return(nil)
				provider.EXPECT().Status(gomock.Any(), "zsh").Return(nil, fmt.Errorf("final status failed"))

				event, err := pkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("final status failed"))
			})

			It("Should fail if desired state is not reached", func(ctx context.Context) {
				pkg.prop.Ensure = EnsureAbsent
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}
				finalState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

				provider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				provider.EXPECT().Uninstall(gomock.Any(), "zsh").Return(nil)
				provider.EXPECT().Status(gomock.Any(), "zsh").Return(finalState, nil)

				event, err := pkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("failed to reach desired state"))
			})

			Context("with health check", func() {
				It("Should succeed when health check passes", func(ctx context.Context) {
					pkg.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_disk -w 20%",
					}
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk", "-w", "20%").
						Return([]byte("DISK OK"), []byte{}, 0, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeFalse())
					Expect(result.Error).To(BeEmpty())
				})

				It("Should fail when health check returns warning", func(ctx context.Context) {
					pkg.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_disk -w 20%",
					}
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk", "-w", "20%").
						Return([]byte("DISK WARNING - 15% free"), []byte{}, 1, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(Equal("health check status \"WARNING\""))
					Expect(result.HealthCheck).ToNot(BeNil())
					Expect(result.HealthCheck.Status).To(Equal(model.HealthCheckWarning))
				})

				It("Should fail when health check returns critical", func(ctx context.Context) {
					pkg.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_disk -w 20%",
					}
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk", "-w", "20%").
						Return([]byte("DISK CRITICAL - 5% free"), []byte{}, 2, nil)

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(Equal("health check status \"CRITICAL\""))
					Expect(result.HealthCheck).ToNot(BeNil())
					Expect(result.HealthCheck.Status).To(Equal(model.HealthCheckCritical))
				})

				It("Should fail when health check command execution fails", func(ctx context.Context) {
					pkg.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/lib/nagios/plugins/check_disk -w 20%",
					}
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/lib/nagios/plugins/check_disk", "-w", "20%").
						Return(nil, nil, 0, fmt.Errorf("command not found"))

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(ContainSubstring("command not found"))
				})

				It("Should not run health check when not configured", func(ctx context.Context) {
					pkg.prop.HealthCheck = nil
					state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

					provider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)
					// No runner.EXPECT() for health check command

					result, err := pkg.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeFalse())
					Expect(result.HealthCheck).To(BeNil())
				})
			})
		})

		Describe("Apply in noop mode", func() {
			var noopMgr *modelmocks.MockManager
			var noopPkg *Type
			var noopProvider *MockPackageProvider

			BeforeEach(func(ctx context.Context) {
				noopMgr, _ = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner := modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)
				noopProvider = NewMockPackageProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()

				noopFactory := modelmocks.NewMockProviderFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.PackageTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts).Return(true, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				noopProperties := &model.PackageResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "zsh",
						Ensure:   "present",
						Provider: "noop-test",
					},
				}
				var err error
				noopPkg, err = New(ctx, noopMgr, *noopProperties)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should not install when package is absent", func(ctx context.Context) {
				noopPkg.prop.Ensure = EnsurePresent
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Install call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have installed version present"))
			})

			It("Should not uninstall when package is present", func(ctx context.Context) {
				noopPkg.prop.Ensure = EnsureAbsent
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Uninstall call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have uninstalled"))
			})

			It("Should not upgrade when ensure is latest", func(ctx context.Context) {
				noopPkg.prop.Ensure = EnsureLatest
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Upgrade call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have upgraded to latest"))
			})

			It("Should not install latest when package is absent", func(ctx context.Context) {
				noopPkg.prop.Ensure = EnsureLatest
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Install call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have installed latest"))
			})

			It("Should not upgrade to specific version", func(ctx context.Context) {
				noopPkg.prop.Ensure = "2.0.0"
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Upgrade call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have upgraded to 2.0.0"))
			})

			It("Should not downgrade to specific version", func(ctx context.Context) {
				noopPkg.prop.Ensure = "1.0.0"
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "2.0.0"}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Downgrade call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have downgraded to 1.0.0"))
			})

			It("Should not change when already in desired state", func(ctx context.Context) {
				noopPkg.prop.Ensure = EnsurePresent
				state := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: "1.0.0"}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(state, nil)

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeFalse())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(BeEmpty())
			})

			It("Should not install specific version when package is absent", func(ctx context.Context) {
				noopPkg.prop.Ensure = "2.0.0"
				initialState := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh", Ensure: EnsureAbsent}}

				noopProvider.EXPECT().Status(gomock.Any(), "zsh").Return(initialState, nil)
				// No Install call expected

				result, err := noopPkg.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have installed version 2.0.0"))
			})
		})

		Describe("Info", func() {
			It("Should fail if no suitable factory", func() {
				factory.EXPECT().IsManageable(facts).Return(false, nil)

				_, err := pkg.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotManageable))
			})

			It("Should fail for unknown factory", func() {
				pkg.prop.Provider = "unknown"
				_, err := pkg.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotFound))
			})

			It("Should handle info failures", func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil)
				provider.EXPECT().Status(gomock.Any(), "zsh").Return(nil, fmt.Errorf("cant execute status command"))

				nfo, err := pkg.Info(context.Background())
				Expect(err).To(Equal(fmt.Errorf("cant execute status command")))
				Expect(nfo).To(BeNil())
			})

			It("Should call status on the provider", func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil)

				res := &model.PackageState{CommonResourceState: model.CommonResourceState{Name: "zsh"}}
				provider.EXPECT().Status(gomock.Any(), "zsh").Return(res, nil)

				nfo, err := pkg.Info(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(nfo).ToNot(BeNil())
				Expect(nfo).To(Equal(res))
			})
		})
	})
})
