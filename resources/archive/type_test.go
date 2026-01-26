// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package archiveresource

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

func TestArchiveResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types/Archive")
}

var _ = Describe("Archive Type", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		mockctl  *gomock.Controller
		provider *MockArchiveProvider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		provider = NewMockArchiveProvider(mockctl)

		provider.EXPECT().Name().Return("mock").AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	Describe("isDesiredState", func() {
		var archive *Type

		BeforeEach(func(ctx context.Context) {
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/test.tar.gz",
					Ensure: model.EnsurePresent,
				},
				Url:   "https://example.com/test.tar.gz",
				Owner: "root",
				Group: "root",
			}
			var err error
			archive, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("state matching",
			func(props *model.ArchiveResourceProperties, meta *model.ArchiveMetadata, stateEnsure string, expected bool) {
				state := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Ensure: stateEnsure},
					Metadata:            meta,
				}
				isStable, _, err := archive.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(isStable).To(Equal(expected))
			},

			// ensure=absent cases
			Entry("absent matches absent",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsureAbsent}},
				&model.ArchiveMetadata{},
				model.EnsureAbsent, true),
			Entry("absent does not match present",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsureAbsent}},
				&model.ArchiveMetadata{ArchiveExists: true},
				model.EnsurePresent, false),

			// creates file checks
			Entry("creates file missing returns false",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Creates: "/opt/app/bin", Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: true, CreatesExists: false},
				model.EnsurePresent, false),
			Entry("creates file exists with cleanup returns true",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Creates: "/opt/app/bin", Cleanup: true, ExtractParent: "/opt", Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: false, CreatesExists: true},
				model.EnsurePresent, true),

			// archive existence with cleanup flag
			Entry("archive missing without cleanup returns false",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Cleanup: false, Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: false},
				model.EnsurePresent, false),
			Entry("archive missing with cleanup returns true",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Cleanup: true, ExtractParent: "/opt", Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: false},
				model.EnsurePresent, true),

			// owner/group checks
			Entry("owner mismatch returns false",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: true, Owner: "nobody", Group: "root"},
				model.EnsurePresent, false),
			Entry("group mismatch returns false",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: true, Owner: "root", Group: "nobody"},
				model.EnsurePresent, false),

			// checksum checks
			Entry("checksum mismatch returns false",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Owner: "root", Group: "root", Checksum: "abc123"},
				&model.ArchiveMetadata{ArchiveExists: true, Owner: "root", Group: "root", Checksum: "def456"},
				model.EnsurePresent, false),
			Entry("checksum match returns true",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Owner: "root", Group: "root", Checksum: "abc123"},
				&model.ArchiveMetadata{ArchiveExists: true, Owner: "root", Group: "root", Checksum: "abc123"},
				model.EnsurePresent, true),
			Entry("empty checksum in props ignores state checksum",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Owner: "root", Group: "root", Checksum: ""},
				&model.ArchiveMetadata{ArchiveExists: true, Owner: "root", Group: "root", Checksum: "abc123"},
				model.EnsurePresent, true),

			// all matching
			Entry("all properties match returns true",
				&model.ArchiveResourceProperties{CommonResourceProperties: model.CommonResourceProperties{Ensure: model.EnsurePresent}, Owner: "root", Group: "root"},
				&model.ArchiveMetadata{ArchiveExists: true, Owner: "root", Group: "root"},
				model.EnsurePresent, true),
		)
	})

	Describe("New", func() {
		It("Should validate properties", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ArchiveResourceProperties{})
			Expect(err).To(MatchError(model.ErrResourceNameRequired))

			_, err = New(ctx, mgr, model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{Name: "/tmp/test.tar.gz"},
			})
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})

		It("Should set alias from properties", func(ctx context.Context) {
			archive, err := New(ctx, mgr, model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/app.tar.gz",
					Ensure: model.EnsurePresent,
					Alias:  "app-archive",
				},
				Url:   "https://example.com/app.tar.gz",
				Owner: "root",
				Group: "root",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(archive).ToNot(BeNil())
			Expect(archive.Base.CommonProperties.Alias).To(Equal("app-archive"))

			event := archive.NewTransactionEvent()
			Expect(event.Alias).To(Equal("app-archive"))
		})

		It("Should fail when cleanup is true but extract_parent is empty", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/app.tar.gz",
					Ensure: model.EnsurePresent,
				},
				Url:     "https://example.com/app.tar.gz",
				Owner:   "root",
				Group:   "root",
				Cleanup: true,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cleanup requires extract_parent"))
		})
	})

	Context("with a prepared provider", func() {
		var factory *MockArchiveFactory
		var archive *Type
		var properties *model.ArchiveResourceProperties
		var err error

		BeforeEach(func(ctx context.Context) {
			factory = NewMockArchiveFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.ArchiveTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})
			properties = &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "/tmp/app.tar.gz",
					Ensure:   model.EnsurePresent,
					Provider: "test",
				},
				Url:           "https://example.com/app.tar.gz",
				Owner:         "root",
				Group:         "root",
				ExtractParent: "/opt",
				Creates:       "/opt/app/bin",
			}
			archive, err = New(ctx, mgr, *properties)
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

				event, err := archive.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Errors).To(ContainElement(ContainSubstring("status failed")))
			})

			Context("when ensure is present", func() {
				It("Should download and extract when archive does not exist", func(ctx context.Context) {
					initialState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: false, CreatesExists: false},
					}
					finalState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: true, Owner: "root", Group: "root"},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should not change when already in desired state", func(ctx context.Context) {
					state := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: true, Owner: "root", Group: "root"},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should re-download when checksum does not match", func(ctx context.Context) {
					archive.prop.Checksum = "newchecksum"
					initialState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: true, Owner: "root", Group: "root", Checksum: "oldchecksum"},
					}
					finalState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: true, Owner: "root", Group: "root", Checksum: "newchecksum"},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should fail if download fails", func(ctx context.Context) {
					initialState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: false, CreatesExists: false},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("download failed"))

					event, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Errors).To(HaveLen(1))
					Expect(event.Errors[0]).To(ContainSubstring("download failed"))
				})

				It("Should fail if extract fails", func(ctx context.Context) {
					initialState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: false, CreatesExists: false},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("extract failed"))

					event, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Errors).To(ContainElement("extract failed"))
				})

				It("Should extract when creates file does not exist", func(ctx context.Context) {
					initialState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: false, Owner: "root", Group: "root", Checksum: "abc123"},
					}
					finalState := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: true, Owner: "root", Group: "root", Checksum: "abc123"},
					}

					archive.prop.Checksum = "abc123"

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
					provider.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

					result, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})
			})

			Context("when ensure is absent", func() {
				BeforeEach(func() {
					archive.prop.Ensure = model.EnsureAbsent
					archive.Base.CommonProperties = archive.prop.CommonResourceProperties
				})

				It("Should not change when archive is already absent", func(ctx context.Context) {
					state := &model.ArchiveState{
						CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
						Metadata:            &model.ArchiveMetadata{ArchiveExists: false},
					}

					provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(state, nil)

					result, err := archive.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})
			})

			It("Should fail if final status check fails", func(ctx context.Context) {
				initialState := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
					Metadata:            &model.ArchiveMetadata{ArchiveExists: false, CreatesExists: false},
				}

				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
				provider.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				provider.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("final status failed"))

				event, err := archive.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Errors).To(ContainElement("final status failed"))
			})

			It("Should fail if desired state is not reached", func(ctx context.Context) {
				initialState := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
					Metadata:            &model.ArchiveMetadata{ArchiveExists: false, CreatesExists: false},
				}
				// Final state still shows creates doesn't exist
				finalState := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
					Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: false, Owner: "root", Group: "root"},
				}

				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
				provider.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				provider.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(finalState, nil)

				event, err := archive.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Errors).To(ContainElement(ContainSubstring("failed to reach desired state")))
			})
		})

		Describe("Apply in noop mode", func() {
			var noopMgr *modelmocks.MockManager
			var noopArchive *Type
			var noopProvider *MockArchiveProvider
			var noopFactory *MockArchiveFactory

			BeforeEach(func(ctx context.Context) {
				noopMgr, _ = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner := modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)
				noopProvider = NewMockArchiveProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()

				noopFactory = NewMockArchiveFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.ArchiveTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				noopProperties := &model.ArchiveResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "/tmp/app.tar.gz",
						Ensure:   model.EnsurePresent,
						Provider: "noop-test",
					},
					Url:           "https://example.com/app.tar.gz",
					Owner:         "root",
					Group:         "root",
					ExtractParent: "/opt",
					Creates:       "/opt/app/bin",
				}
				var err error
				noopArchive, err = New(ctx, noopMgr, *noopProperties)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should not download when archive does not exist", func(ctx context.Context) {
				initialState := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsureAbsent},
					Metadata:            &model.ArchiveMetadata{ArchiveExists: false, CreatesExists: false},
				}

				noopProvider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
				// No Download or Extract calls expected

				result, err := noopArchive.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(ContainSubstring("Would have downloaded"))
				Expect(result.NoopMessage).To(ContainSubstring("Would have extracted"))
			})

			It("Should not change when already in desired state", func(ctx context.Context) {
				state := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
					Metadata:            &model.ArchiveMetadata{ArchiveExists: true, CreatesExists: true, Owner: "root", Group: "root"},
				}

				noopProvider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(state, nil)

				result, err := noopArchive.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeFalse())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(BeEmpty())
			})

			It("Should not remove when ensure is absent", func(ctx context.Context) {
				noopArchive.prop.Ensure = model.EnsureAbsent
				initialState := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz", Ensure: model.EnsurePresent},
					Metadata:            &model.ArchiveMetadata{ArchiveExists: true},
				}

				noopProvider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(initialState, nil)
				// No os.Remove call expected

				result, err := noopArchive.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(ContainSubstring("Would have removed"))
			})
		})

		Describe("Info", func() {
			It("Should fail if IsManageable returns false", func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(false, 1, nil)

				_, err := archive.Info(context.Background())
				Expect(err).To(MatchError(ContainSubstring("not applicable to instance")))
			})

			It("Should handle info failures", func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil)
				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cant execute status command"))

				nfo, err := archive.Info(context.Background())
				Expect(err).To(Equal(fmt.Errorf("cant execute status command")))
				Expect(nfo).To(BeNil())
			})

			It("Should call status on the provider", func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil)

				res := &model.ArchiveState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/app.tar.gz"},
					Metadata:            &model.ArchiveMetadata{},
				}
				provider.EXPECT().Status(gomock.Any(), gomock.Any()).Return(res, nil)

				nfo, err := archive.Info(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(nfo).ToNot(BeNil())
				Expect(nfo).To(Equal(res))
			})
		})
	})
})
