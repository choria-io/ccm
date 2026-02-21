// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffoldresource

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
			})

			It("Should be stable with empty scaffold", func() {
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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeTrue())
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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})

			It("Should not be stable when there are purged files", func() {
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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
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

				stable, err := scaffold.isDesiredState(props, state)
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

				stable, err := scaffold.isDesiredState(props, state)
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

				stable, err := scaffold.isDesiredState(props, state)
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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})

			It("Should not be stable when target exists with purged files", func() {
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

				stable, err := scaffold.isDesiredState(props, state)
				Expect(err).ToNot(HaveOccurred())
				Expect(stable).To(BeFalse())
			})
		})
	})
})
