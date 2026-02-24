// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffoldresource

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

func TestScaffoldResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Scaffold")
}

var _ = Describe("Scaffold Type", func() {
	var (
		facts   = make(map[string]any)
		data    = make(map[string]any)
		mgr     *modelmocks.MockManager
		mockctl *gomock.Controller
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, _ = modelmocks.NewManager(facts, data, false, mockctl)
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("New", func() {
		It("Should fail when name is empty", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).To(MatchError(model.ErrResourceNameRequired))
		})

		It("Should fail when ensure is empty", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/opt/app/scaffold",
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})

		It("Should fail when source is empty", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("source cannot be empty"))
		})

		It("Should fail when name is not an absolute path", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "relative/path",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("absolute path"))
		})

		It("Should fail when name is not canonical", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/../etc/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("canonical"))
		})

		It("Should create a valid scaffold resource", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold).ToNot(BeNil())
			Expect(scaffold.prop.Name).To(Equal("/opt/app/scaffold"))
			Expect(scaffold.prop.Source).To(Equal("https://example.com/scaffold.tar.gz"))
			Expect(scaffold.prop.Engine).To(Equal(model.ScaffoldEngineGo))
		})

		It("Should set alias from properties", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
					Alias:  "my-scaffold",
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold).ToNot(BeNil())
			Expect(scaffold.Base.CommonProperties.Alias).To(Equal("my-scaffold"))

			event := scaffold.NewTransactionEvent()
			Expect(event.Alias).To(Equal("my-scaffold"))
		})

		It("Should set Type to scaffold", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold.prop.Type).To(Equal(model.ScaffoldTypeName))
		})

		It("Should resolve templates in properties", func(ctx context.Context) {
			facts["appname"] = "myapp"
			facts["version"] = "v1.0.0"
			mgr, _ = modelmocks.NewManager(facts, data, false, mockctl)

			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/{{ Facts.appname }}/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/{{ Facts.version }}/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold.prop.Name).To(Equal("/opt/myapp/scaffold"))
			Expect(scaffold.prop.Source).To(Equal("https://example.com/v1.0.0/scaffold.tar.gz"))
		})
	})

	Describe("validate", func() {
		It("Should require engine to be specified", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("engine must be one of"))
		})

		It("Should set jet delimiters when engine is jet", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineJet,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold.prop.LeftDelimiter).To(Equal("[["))
			Expect(scaffold.prop.RightDelimiter).To(Equal("]]"))
		})

		It("Should set go delimiters when engine is go", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold.prop.LeftDelimiter).To(Equal("{{"))
			Expect(scaffold.prop.RightDelimiter).To(Equal("}}"))
		})

		It("Should preserve custom delimiters", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source:         "https://example.com/scaffold.tar.gz",
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "<%",
				RightDelimiter: "%>",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold.prop.LeftDelimiter).To(Equal("<%"))
			Expect(scaffold.prop.RightDelimiter).To(Equal("%>"))
		})

		It("Should skip validation when SkipValidate is true", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:         "", // Invalid but should be skipped
					Ensure:       model.EnsurePresent,
					SkipValidate: true,
				},
				Source: "",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold).ToNot(BeNil())
		})
	})

	Describe("Provider", func() {
		It("Should return empty string when no provider is set", func(ctx context.Context) {
			scaffold, err := New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(scaffold.Provider()).To(Equal(""))
		})
	})

	Context("with a prepared provider", func() {
		var (
			factory  *modelmocks.MockProviderFactory
			scaffold *Type
			provider *MockScaffoldProvider
		)

		BeforeEach(func(ctx context.Context) {
			provider = NewMockScaffoldProvider(mockctl)
			provider.EXPECT().Name().Return("mock").AnyTimes()

			factory = modelmocks.NewMockProviderFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.ScaffoldTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})

			runner := modelmocks.NewMockCommandRunner(mockctl)
			mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)

			var err error
			scaffold, err = New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "/opt/app/scaffold",
					Ensure:   model.EnsurePresent,
					Provider: "test",
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())

			registry.Clear()
			registry.MustRegister(factory)
		})

		Describe("Apply", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()
			})

			It("Should fail if initial status check fails", func(ctx context.Context) {
				provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("status failed"))

				event, err := scaffold.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Failed).To(BeTrue())
				Expect(event.Errors).To(ContainElement(ContainSubstring("status failed")))
			})

			Context("ensure present", func() {
				It("Should not make changes when already stable", func(ctx context.Context) {
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt", "file2.txt"},
							Changed: []string{},
							Purged:  []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.Noop).To(BeFalse())
				})

				It("Should scaffold when files need changing", func(ctx context.Context) {
					initialState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt"},
							Changed: []string{"file2.txt"},
							Purged:  []string{},
						},
					}
					finalState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt", "file2.txt"},
							Changed: []string{},
							Purged:  []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Scaffold(gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeFalse())
				})

				It("Should scaffold when files need purging with purge enabled", func(ctx context.Context) {
					scaffold.prop.Purge = true
					initialState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt"},
							Changed: []string{},
							Purged:  []string{"stale.txt"},
						},
					}
					finalState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt"},
							Changed: []string{},
							Purged:  []string{"stale.txt"},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Scaffold(gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should fail when Scaffold returns an error", func(ctx context.Context) {
					initialState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{},
							Changed: []string{"file1.txt"},
							Purged:  []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Scaffold(gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, fmt.Errorf("scaffold failed"))

					event, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Failed).To(BeTrue())
					Expect(event.Errors).To(ContainElement(ContainSubstring("scaffold failed")))
				})

				It("Should fail when desired state is not reached after apply", func(ctx context.Context) {
					initialState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{},
							Changed: []string{"file1.txt"},
							Purged:  []string{},
						},
					}
					finalState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{},
							Changed: []string{},
							Purged:  []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Scaffold(gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(finalState, nil)

					event, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Failed).To(BeTrue())
					Expect(event.Errors).To(ContainElement(ContainSubstring(model.ErrDesiredStateFailed.Error())))
				})
			})

			Context("ensure absent", func() {
				BeforeEach(func() {
					scaffold.prop.Ensure = model.EnsureAbsent
				})

				It("Should not make changes when target does not exist", func(ctx context.Context) {
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: false,
							Stable:       []string{},
							Changed:      []string{},
							Purged:       []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.Noop).To(BeFalse())
				})

				It("Should remove scaffold files when present", func(ctx context.Context) {
					initialState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: true,
							Stable:       []string{"file1.txt"},
							Changed:      []string{"file2.txt"},
							Purged:       []string{},
						},
					}
					finalState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: true,
							Stable:       []string{},
							Changed:      []string{},
							Purged:       []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Remove(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeFalse())
				})

				It("Should fail when Remove returns an error", func(ctx context.Context) {
					initialState := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: true,
							Stable:       []string{"file1.txt"},
							Changed:      []string{},
							Purged:       []string{},
						},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Remove(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("remove failed"))

					event, err := scaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Failed).To(BeTrue())
					Expect(event.Errors).To(ContainElement(ContainSubstring("remove failed")))
				})
			})
		})

		Describe("Apply in noop mode", func() {
			var (
				noopMgr      *modelmocks.MockManager
				noopScaffold *Type
				noopProvider *MockScaffoldProvider
			)

			BeforeEach(func(ctx context.Context) {
				noopMgr, _ = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner := modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)

				noopProvider = NewMockScaffoldProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()

				noopFactory := modelmocks.NewMockProviderFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.ScaffoldTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				var err error
				noopScaffold, err = New(ctx, noopMgr, model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "/opt/app/scaffold",
						Ensure:   model.EnsurePresent,
						Provider: "noop-test",
					},
					Source: "https://example.com/scaffold.tar.gz",
					Engine: model.ScaffoldEngineGo,
				})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("ensure present", func() {
				It("Should not change when already in desired state", func(ctx context.Context) {
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt", "file2.txt"},
							Changed: []string{},
							Purged:  []string{},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(BeEmpty())
				})

				It("Should report changes needed without applying them", func(ctx context.Context) {
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt"},
							Changed: []string{"file2.txt", "file3.txt"},
							Purged:  []string{},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(Equal("Would have changed 2 scaffold files"))
				})

				It("Should include purged files in count when purge is enabled", func(ctx context.Context) {
					noopScaffold.prop.Purge = true
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt"},
							Changed: []string{"file2.txt"},
							Purged:  []string{"stale1.txt", "stale2.txt"},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(Equal("Would have changed 3 scaffold files"))
				})

				It("Should not include purged files in count when purge is disabled", func(ctx context.Context) {
					noopScaffold.prop.Purge = false
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							Stable:  []string{"file1.txt"},
							Changed: []string{"file2.txt"},
							Purged:  []string{"stale1.txt", "stale2.txt"},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(Equal("Would have changed 1 scaffold files"))
				})
			})

			Context("ensure absent", func() {
				BeforeEach(func() {
					noopScaffold.prop.Ensure = model.EnsureAbsent
				})

				It("Should not change when target does not exist", func(ctx context.Context) {
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: false,
							Stable:       []string{},
							Changed:      []string{},
							Purged:       []string{},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(BeEmpty())
				})

				It("Should report removal needed without applying it", func(ctx context.Context) {
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: true,
							Stable:       []string{"file1.txt", "file2.txt"},
							Changed:      []string{"file3.txt"},
							Purged:       []string{},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(Equal("Would have removed 3 scaffold files"))
				})

				It("Should include purged files in removal count when purge is enabled", func(ctx context.Context) {
					noopScaffold.prop.Purge = true
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: true,
							Stable:       []string{"file1.txt"},
							Changed:      []string{},
							Purged:       []string{"extra1.txt", "extra2.txt"},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(Equal("Would have removed 3 scaffold files"))
				})

				It("Should not include purged files in removal count when purge is disabled", func(ctx context.Context) {
					noopScaffold.prop.Purge = false
					state := &model.ScaffoldState{
						Metadata: &model.ScaffoldMetadata{
							TargetExists: true,
							Stable:       []string{"file1.txt"},
							Changed:      []string{},
							Purged:       []string{"extra1.txt", "extra2.txt"},
						},
					}

					noopProvider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := noopScaffold.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Noop).To(BeTrue())
					Expect(result.NoopMessage).To(Equal("Would have removed 1 scaffold files"))
				})
			})
		})

		Describe("Info", func() {
			It("Should fail if no suitable factory", func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(false, 0, nil)

				_, err := scaffold.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotManageable))
			})

			It("Should fail for unknown factory", func() {
				scaffold.prop.Provider = "unknown"
				_, err := scaffold.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotFound))
			})

			It("Should call status on the provider", func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil)

				res := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Name:   "/opt/app/scaffold",
						Stable: []string{"file1.txt"},
					},
				}
				provider.EXPECT().Status(gomock.Any(), gomock.Any(), gomock.Any()).Return(res, nil)

				nfo, err := scaffold.Info(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(nfo).To(Equal(res))
			})
		})
	})

	Describe("isDesiredState", func() {
		var scaffold *Type

		BeforeEach(func(ctx context.Context) {
			var err error
			scaffold, err = New(ctx, mgr, model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/opt/app/scaffold",
					Ensure: model.EnsurePresent,
				},
				Source: "https://example.com/scaffold.tar.gz",
				Engine: model.ScaffoldEngineGo,
			})
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("EnsurePresent", func() {
			It("Should be stable when no changes and no purged files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{"file1.txt", "file2.txt"},
						Changed: []string{},
						Purged:  []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
			})

			It("Should not be stable with empty scaffold", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{},
						Changed: []string{},
						Purged:  []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})

			It("Should not be stable when there are changed files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{"file1.txt"},
						Changed: []string{"file2.txt"},
						Purged:  []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})

			It("Should be stable when there are purged files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{"file1.txt"},
						Changed: []string{},
						Purged:  []string{"old_file.txt"},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
			})

			It("Should not be stable when there are both changed and purged files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{},
						Changed: []string{"new_file.txt"},
						Purged:  []string{"old_file.txt"},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})
		})

		Describe("EnsurePresent post-apply", func() {
			It("Should be desired when changed files exist", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{},
						Changed: []string{"file1.txt"},
						Purged:  []string{},
					},
				}

				desired, err := scaffold.isDesiredState(props, state, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(desired).To(BeTrue())
			})

			It("Should be desired when stable files exist", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{"file1.txt"},
						Changed: []string{},
						Purged:  []string{},
					},
				}

				desired, err := scaffold.isDesiredState(props, state, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(desired).To(BeTrue())
			})

			It("Should be desired when purged files exist", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{"x.txt"},
						Changed: []string{},
						Purged:  []string{"old.txt"},
					},
				}

				desired, err := scaffold.isDesiredState(props, state, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(desired).To(BeTrue())
			})

			It("Should not be desired when no files managed", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsurePresent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						Stable:  []string{},
						Changed: []string{},
						Purged:  []string{},
					},
				}

				desired, err := scaffold.isDesiredState(props, state, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(desired).To(BeFalse())
			})
		})

		Describe("EnsureAbsent", func() {
			It("Should be stable when target does not exist", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsureAbsent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						TargetExists: false,
						Stable:       []string{},
						Changed:      []string{},
						Purged:       []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
			})

			It("Should be stable when target exists but has no scaffold files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsureAbsent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						TargetExists: true,
						Stable:       []string{},
						Changed:      []string{},
						Purged:       []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
			})

			It("Should not be stable when target exists with stable files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsureAbsent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						TargetExists: true,
						Stable:       []string{"file1.txt"},
						Changed:      []string{},
						Purged:       []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})

			It("Should not be stable when target exists with changed files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsureAbsent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						TargetExists: true,
						Stable:       []string{},
						Changed:      []string{"file1.txt"},
						Purged:       []string{},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})

			It("Should be stable when target exists with only purged files", func() {
				props := &model.ScaffoldResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Ensure: model.EnsureAbsent,
					},
				}
				state := &model.ScaffoldState{
					Metadata: &model.ScaffoldMetadata{
						TargetExists: true,
						Stable:       []string{},
						Changed:      []string{},
						Purged:       []string{"file1.txt"},
					},
				}

				stable, err := scaffold.isDesiredState(props, state, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
			})
		})
	})
})
