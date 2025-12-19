// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package fileresource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestFileResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/File")
}

func checksum(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

var _ = Describe("File Type", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		mockctl  *gomock.Controller
		provider *MockFileProvider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		provider = NewMockFileProvider(mockctl)

		provider.EXPECT().Name().Return("mock").AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	})

	Describe("New", func() {
		It("Should validate properties", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.FileResourceProperties{})
			Expect(err).To(MatchError(model.ErrResourceNameRequired))

			_, err = New(ctx, mgr, model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{Name: "/tmp/foo"},
			})
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})

		It("Should validate mode is valid octal", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/foo",
					Ensure: model.EnsurePresent,
				},
				Owner: "root",
				Group: "root",
				Mode:  "invalid",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not a valid octal number"))
		})

		It("Should reject mode exceeding 0777", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/foo",
					Ensure: model.EnsurePresent,
				},
				Owner: "root",
				Group: "root",
				Mode:  "1777",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exceeds maximum value"))
		})

		DescribeTable("valid mode formats",
			func(ctx context.Context, mode string) {
				_, err := New(ctx, mgr, model.FileResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   "/tmp/foo",
						Ensure: model.EnsurePresent,
					},
					Owner:    "root",
					Group:    "root",
					Mode:     mode,
					Contents: "test",
				})
				Expect(err).ToNot(HaveOccurred())
			},
			Entry("octal with leading zero", "0644"),
			Entry("octal without leading zero", "644"),
			Entry("octal with 0o prefix", "0o644"),
			Entry("octal with 0O prefix", "0O755"),
		)
	})

	Describe("isDesiredState", func() {
		var file *Type

		BeforeEach(func(ctx context.Context) {
			properties := &model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/test-file",
					Ensure: model.EnsurePresent,
				},
				Owner:    "root",
				Group:    "root",
				Mode:     "0644",
				Contents: "test content",
			}
			var err error
			file, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should return true when ensure=absent and file is absent", func() {
			file.prop.Ensure = model.EnsureAbsent
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
				Metadata:            &model.FileMetadata{},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeTrue())
		})

		It("Should return false when ensure=absent but file is present", func() {
			file.prop.Ensure = model.EnsureAbsent
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
				Metadata:            &model.FileMetadata{},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeFalse())
		})

		It("Should return true when all properties match", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
				Metadata: &model.FileMetadata{
					Owner:    "root",
					Group:    "root",
					Mode:     "0644",
					Checksum: checksum("test content"),
				},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeTrue())
		})

		It("Should return false when content checksum differs", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
				Metadata: &model.FileMetadata{
					Owner:    "root",
					Group:    "root",
					Mode:     "0644",
					Checksum: checksum("different content"),
				},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeFalse())
		})

		It("Should return false when owner differs", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
				Metadata: &model.FileMetadata{
					Owner:    "nobody",
					Group:    "root",
					Mode:     "0644",
					Checksum: checksum("test content"),
				},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeFalse())
		})

		It("Should return false when group differs", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
				Metadata: &model.FileMetadata{
					Owner:    "root",
					Group:    "nobody",
					Mode:     "0644",
					Checksum: checksum("test content"),
				},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeFalse())
		})

		It("Should return false when mode differs", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
				Metadata: &model.FileMetadata{
					Owner:    "root",
					Group:    "root",
					Mode:     "0755",
					Checksum: checksum("test content"),
				},
			}
			isStable, _, err := file.isDesiredState(file.prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(isStable).To(BeFalse())
		})

		Context("with source file", func() {
			var sourceFile string
			var sourceContent string

			BeforeEach(func() {
				sourceContent = "source file content"
				tmpDir := GinkgoT().TempDir()
				sourceFile = filepath.Join(tmpDir, "source.txt")
				err := os.WriteFile(sourceFile, []byte(sourceContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				file.prop.Source = sourceFile
				file.prop.Contents = ""
			})

			It("Should return true when file checksum matches source file", func() {
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum(sourceContent),
					},
				}
				isStable, _, err := file.isDesiredState(file.prop, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(isStable).To(BeTrue())
			})

			It("Should return false when file checksum differs from source file", func() {
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum("different content"),
					},
				}
				isStable, _, err := file.isDesiredState(file.prop, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(isStable).To(BeFalse())
			})

			It("Should return error when source file does not exist", func() {
				file.prop.Source = "/nonexistent/source/file.txt"
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum("some content"),
					},
				}
				_, _, err := file.isDesiredState(file.prop, state)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no such file or directory"))
			})
		})

		Context("with directory ensure", func() {
			BeforeEach(func() {
				file.prop.Ensure = model.FileEnsureDirectory
				file.prop.Contents = ""
			})

			It("Should return true when directory exists with matching properties", func() {
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.FileEnsureDirectory},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum(""),
					},
				}
				isStable, _, err := file.isDesiredState(file.prop, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(isStable).To(BeTrue())
			})

			It("Should return false when directory does not exist", func() {
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
					Metadata:            &model.FileMetadata{},
				}
				isStable, _, err := file.isDesiredState(file.prop, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(isStable).To(BeFalse())
			})

			It("Should return false when path is a file instead of directory", func() {
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum("some content"),
					},
				}
				isStable, _, err := file.isDesiredState(file.prop, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(isStable).To(BeFalse())
			})
		})
	})

	Context("with a prepared provider", func() {
		var factory *modelmocks.MockProviderFactory
		var file *Type
		var properties *model.FileResourceProperties
		var err error

		BeforeEach(func(ctx context.Context) {
			factory = modelmocks.NewMockProviderFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.FileTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})
			properties = &model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "/tmp/testfile",
					Ensure:   model.EnsurePresent,
					Provider: "test",
				},
				Owner:    "root",
				Group:    "root",
				Mode:     "0644",
				Contents: "file content",
			}
			file, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())

			registry.Clear()
			registry.MustRegister(factory)
		})

		Describe("Apply", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil).AnyTimes()
			})

			It("Should fail if initial status check fails", func(ctx context.Context) {
				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(nil, fmt.Errorf("status failed"))

				event, err := file.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("status failed"))
			})

			Context("when ensure is present", func() {
				BeforeEach(func() {
					file.prop.Ensure = model.EnsurePresent
					file.Base.Ensure = model.EnsurePresent
				})

				It("Should create file when absent", func(ctx context.Context) {
					initialState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
						Metadata:            &model.FileMetadata{},
					}
					finalState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
					provider.EXPECT().Store(gomock.Any(), "/tmp/testfile", []byte("file content"), "", "root", "root", "0644").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(finalState, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal(model.EnsurePresent))
				})

				It("Should update file when content differs", func(ctx context.Context) {
					initialState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("old content"),
						},
					}
					finalState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
					provider.EXPECT().Store(gomock.Any(), "/tmp/testfile", []byte("file content"), "", "root", "root", "0644").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(finalState, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should update file when owner differs", func(ctx context.Context) {
					initialState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "nobody",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}
					finalState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
					provider.EXPECT().Store(gomock.Any(), "/tmp/testfile", []byte("file content"), "", "root", "root", "0644").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(finalState, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
				})

				It("Should not change when file already matches", func(ctx context.Context) {
					state := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(state, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should fail if store fails", func(ctx context.Context) {
					initialState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
						Metadata:            &model.FileMetadata{},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
					provider.EXPECT().Store(gomock.Any(), "/tmp/testfile", []byte("file content"), "", "root", "root", "0644").Return(fmt.Errorf("store failed"))

					event, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("store failed"))
				})
			})

			Context("when ensure is absent", func() {
				BeforeEach(func() {
					file.prop.Ensure = model.EnsureAbsent
				})

				It("Should not change when file already absent", func(ctx context.Context) {
					state := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
						Metadata:            &model.FileMetadata{},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(state, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})
			})

			Context("when ensure is directory", func() {
				BeforeEach(func() {
					file.prop.Ensure = model.FileEnsureDirectory
					file.Base.Ensure = model.FileEnsureDirectory
					file.prop.Contents = ""
				})

				It("Should create directory when absent", func(ctx context.Context) {
					initialState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
						Metadata:            &model.FileMetadata{},
					}
					finalState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.FileEnsureDirectory},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum(""),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
					provider.EXPECT().CreateDirectory(gomock.Any(), "/tmp/testfile", "root", "root", "0644").Return(nil)
					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(finalState, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeTrue())
					Expect(result.Ensure).To(Equal(model.FileEnsureDirectory))
				})

				It("Should not change when directory already exists", func(ctx context.Context) {
					state := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.FileEnsureDirectory},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum(""),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(state, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Changed).To(BeFalse())
				})

				It("Should fail if CreateDirectory fails", func(ctx context.Context) {
					initialState := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
						Metadata:            &model.FileMetadata{},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
					provider.EXPECT().CreateDirectory(gomock.Any(), "/tmp/testfile", "root", "root", "0644").Return(fmt.Errorf("mkdir failed"))

					event, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(event.Error).To(ContainSubstring("mkdir failed"))
				})
			})

			It("Should fail if final status check fails", func(ctx context.Context) {
				file.prop.Ensure = model.EnsurePresent
				initialState := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
					Metadata:            &model.FileMetadata{},
				}

				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
				provider.EXPECT().Store(gomock.Any(), "/tmp/testfile", []byte("file content"), "", "root", "root", "0644").Return(nil)
				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(nil, fmt.Errorf("final status failed"))

				event, err := file.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("final status failed"))
			})

			It("Should fail if desired state is not reached", func(ctx context.Context) {
				file.prop.Ensure = model.EnsurePresent
				initialState := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
					Metadata:            &model.FileMetadata{},
				}
				finalState := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
					Metadata:            &model.FileMetadata{},
				}

				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(initialState, nil)
				provider.EXPECT().Store(gomock.Any(), "/tmp/testfile", []byte("file content"), "", "root", "root", "0644").Return(nil)
				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(finalState, nil)

				event, err := file.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Error).To(ContainSubstring("failed to reach desired state"))
			})

			Context("with health check", func() {
				It("Should succeed when health check passes", func(ctx context.Context) {
					file.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/bin/test -f /tmp/testfile",
					}
					state := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/bin/test", "-f", "/tmp/testfile").
						Return([]byte("OK"), []byte{}, 0, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeFalse())
				})

				It("Should fail when health check fails", func(ctx context.Context) {
					file.prop.HealthCheck = &model.CommonHealthCheck{
						Command: "/usr/bin/test -f /tmp/testfile",
					}
					state := &model.FileState{
						CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
						Metadata: &model.FileMetadata{
							Owner:    "root",
							Group:    "root",
							Mode:     "0644",
							Checksum: checksum("file content"),
						},
					}

					provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(state, nil)
					runner.EXPECT().Execute(gomock.Any(), "/usr/bin/test", "-f", "/tmp/testfile").
						Return([]byte("CRITICAL"), []byte{}, 2, nil)

					result, err := file.Apply(ctx)
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Failed).To(BeTrue())
					Expect(result.Error).To(ContainSubstring("health check status"))
				})
			})
		})

		Describe("Apply in noop mode", func() {
			var noopMgr *modelmocks.MockManager
			var noopFile *Type
			var noopProvider *MockFileProvider

			BeforeEach(func(ctx context.Context) {
				noopMgr, _ = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner := modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)
				noopProvider = NewMockFileProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()

				noopFactory := modelmocks.NewMockProviderFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.FileTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts).Return(true, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				noopProperties := &model.FileResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "/tmp/noopfile",
						Ensure:   model.EnsurePresent,
						Provider: "noop-test",
					},
					Owner:    "root",
					Group:    "root",
					Mode:     "0644",
					Contents: "noop content",
				}
				var err error
				noopFile, err = New(ctx, noopMgr, *noopProperties)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should not create file when absent", func(ctx context.Context) {
				noopFile.prop.Ensure = model.EnsurePresent
				initialState := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
					Metadata:            &model.FileMetadata{},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "/tmp/noopfile").Return(initialState, nil)
				// No Store call expected

				result, err := noopFile.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have created the file"))
			})

			It("Should not remove file when present", func(ctx context.Context) {
				noopFile.prop.Ensure = model.EnsureAbsent
				initialState := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum("existing content"),
					},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "/tmp/noopfile").Return(initialState, nil)
				// No os.Remove call expected (it's not through provider)

				result, err := noopFile.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have removed the file"))
			})

			It("Should not change when already in desired state", func(ctx context.Context) {
				noopFile.prop.Ensure = model.EnsurePresent
				state := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsurePresent},
					Metadata: &model.FileMetadata{
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Checksum: checksum("noop content"),
					},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "/tmp/noopfile").Return(state, nil)

				result, err := noopFile.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeFalse())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(BeEmpty())
			})

			It("Should not create directory when absent", func(ctx context.Context) {
				noopFile.prop.Ensure = model.FileEnsureDirectory
				noopFile.prop.Contents = ""
				initialState := &model.FileState{
					CommonResourceState: model.CommonResourceState{Ensure: model.EnsureAbsent},
					Metadata:            &model.FileMetadata{},
				}

				noopProvider.EXPECT().Status(gomock.Any(), "/tmp/noopfile").Return(initialState, nil)
				// No CreateDirectory call expected

				result, err := noopFile.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Changed).To(BeTrue())
				Expect(result.Noop).To(BeTrue())
				Expect(result.NoopMessage).To(Equal("Would have created directory"))
			})
		})

		Describe("Info", func() {
			It("Should fail if no suitable factory", func() {
				factory.EXPECT().IsManageable(facts).Return(false, nil)

				_, err := file.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotManageable))
			})

			It("Should fail for unknown factory", func() {
				file.prop.Provider = "unknown"
				_, err := file.Info(context.Background())
				Expect(err).To(MatchError(model.ErrProviderNotFound))
			})

			It("Should handle info failures", func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil)
				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(nil, fmt.Errorf("cant execute status command"))

				nfo, err := file.Info(context.Background())
				Expect(err).To(Equal(fmt.Errorf("cant execute status command")))
				Expect(nfo).To(BeNil())
			})

			It("Should call status on the provider", func() {
				factory.EXPECT().IsManageable(facts).Return(true, nil)

				res := &model.FileState{
					CommonResourceState: model.CommonResourceState{Name: "/tmp/testfile"},
					Metadata:            &model.FileMetadata{},
				}
				provider.EXPECT().Status(gomock.Any(), "/tmp/testfile").Return(res, nil)

				nfo, err := file.Info(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(nfo).ToNot(BeNil())
				Expect(nfo).To(Equal(res))
			})
		})

		Describe("Accessor methods", func() {
			It("Should return empty provider before selection", func() {
				Expect(file.Provider()).To(BeEmpty())
			})
		})
	})
})
