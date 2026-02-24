// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package choriascaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"github.com/choria-io/ccm/templates"
)

func TestChoriaScaffoldProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Scaffold/ChoriaScaffold")
}

var _ = Describe("Choria Scaffold Provider", func() {
	var (
		mockctl  *gomock.Controller
		mocklog  *modelmocks.MockLogger
		provider *Provider
		err      error
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mocklog = modelmocks.NewMockLogger(mockctl)

		mocklog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mocklog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mocklog.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
		mocklog.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

		provider, err = NewChoriaProvider(mocklog)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("choria"))
		})
	})

	Describe("NewChoriaProvider", func() {
		It("Should create a provider with the given logger", func() {
			p, err := NewChoriaProvider(mocklog)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.log).To(Equal(mocklog))
		})
	})

	Describe("logger adapter", func() {
		It("Should forward Debugf to Debug", func() {
			l := modelmocks.NewMockLogger(mockctl)
			l.EXPECT().Debug(fmt.Sprintf("hello %s %d", "world", 42))

			adapter := &logger{log: l}
			adapter.Debugf("hello %s %d", "world", 42)
		})

		It("Should forward Infof to Info", func() {
			l := modelmocks.NewMockLogger(mockctl)
			l.EXPECT().Info(fmt.Sprintf("count: %d", 5))

			adapter := &logger{log: l}
			adapter.Infof("count: %d", 5)
		})
	})

	Describe("Status", func() {
		var (
			tmpDir    string
			sourceDir string
			targetDir string
			env       *templates.Env
		)

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
			sourceDir = filepath.Join(tmpDir, "source")
			targetDir = filepath.Join(tmpDir, "target")

			err := os.MkdirAll(sourceDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			env = &templates.Env{
				Facts: map[string]any{
					"hostname": "testhost",
				},
				Data: map[string]any{
					"app_name": "myapp",
				},
			}
		})

		It("Should report target does not exist", func(ctx context.Context) {
			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source: sourceDir,
				Engine: model.ScaffoldEngineGo,
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Ensure).To(Equal(model.EnsurePresent))
			Expect(state.Metadata.TargetExists).To(BeFalse())
			Expect(state.Metadata.Name).To(Equal(targetDir))
			Expect(state.Metadata.Provider).To(Equal(ProviderName))
		})

		It("Should return stable files when target matches source", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("static content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("static content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Ensure).To(Equal(model.EnsurePresent))
			Expect(state.Metadata.TargetExists).To(BeTrue())
			Expect(state.Metadata.Stable).To(ContainElement(filepath.Join(targetDir, "config.txt")))
			Expect(state.Metadata.Changed).To(BeEmpty())
		})

		It("Should return changed files when target differs from source", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("new content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("old content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Metadata.Changed).To(ContainElement(filepath.Join(targetDir, "config.txt")))
			Expect(state.Metadata.Stable).To(BeEmpty())
		})

		It("Should return changed files when file needs to be added", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "newfile.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Metadata.Changed).To(ContainElement(filepath.Join(targetDir, "newfile.txt")))
		})

		It("Should expand templates in source files", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("hostname: [[ facts.hostname ]]"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("hostname: testhost"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineJet,
				LeftDelimiter:  "[[",
				RightDelimiter: "]]",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Metadata.Stable).To(ContainElement(filepath.Join(targetDir, "config.txt")))
		})

		It("Should handle nested directories", func(ctx context.Context) {
			nestedDir := filepath.Join(sourceDir, "subdir")
			err := os.MkdirAll(nestedDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(nestedDir, "nested.txt"), []byte("nested content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			targetNestedDir := filepath.Join(targetDir, "subdir")
			err = os.MkdirAll(targetNestedDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetNestedDir, "nested.txt"), []byte("nested content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Metadata.Stable).To(ContainElement(filepath.Join(targetDir, "subdir/nested.txt")))
		})

		It("Should work with jet engine", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("static content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("static content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineJet,
				LeftDelimiter:  "[[",
				RightDelimiter: "]]",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Metadata.Engine).To(Equal(model.ScaffoldEngineJet))
			Expect(state.Metadata.Stable).To(ContainElement(filepath.Join(targetDir, "config.txt")))
		})

		It("Should return error for unknown engine", func(ctx context.Context) {
			err := os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source: sourceDir,
				Engine: "unknown",
			}

			_, err = provider.Status(ctx, env, prop)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown scaffold engine"))
		})

		It("Should set metadata fields correctly", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state).ToNot(BeNil())
			Expect(state.Metadata.Name).To(Equal(targetDir))
			Expect(state.Metadata.Provider).To(Equal(ProviderName))
			Expect(state.Metadata.Engine).To(Equal(model.ScaffoldEngineGo))
			Expect(state.Metadata.TargetExists).To(BeTrue())
		})

		It("Should detect files to purge in target not in source", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "keep.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "keep.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "extra.txt"), []byte("extra"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Stable).To(ContainElement(filepath.Join(targetDir, "keep.txt")))
			Expect(state.Metadata.Purged).To(ContainElement(filepath.Join(targetDir, "extra.txt")))
		})

		It("Should filter non-existent files from status when ensure is absent", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "managed.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// target exists but managed file has been removed, only unrelated file remains
			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "unrelated.txt"), []byte("extra"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsureAbsent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.TargetExists).To(BeTrue())
			Expect(state.Metadata.Changed).To(BeEmpty())
			Expect(state.Metadata.Stable).To(BeEmpty())
			Expect(state.Metadata.Purged).To(ContainElement(filepath.Join(targetDir, "unrelated.txt")))
		})

		It("Should not filter files from status when ensure is present", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "managed.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// target exists but managed file is missing
			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "unrelated.txt"), []byte("extra"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Changed).To(ContainElement(filepath.Join(targetDir, "managed.txt")))
		})

		It("Should keep existing files in status when ensure is absent", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "managed.txt"), []byte("new content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// target exists with managed file that differs from source
			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(targetDir, "managed.txt"), []byte("old content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsureAbsent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Status(ctx, env, prop)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Changed).To(ContainElement(filepath.Join(targetDir, "managed.txt")))
		})
	})

	Describe("Scaffold", func() {
		var (
			tmpDir    string
			sourceDir string
			targetDir string
			env       *templates.Env
		)

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
			sourceDir = filepath.Join(tmpDir, "source")
			targetDir = filepath.Join(tmpDir, "target")

			err := os.MkdirAll(sourceDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			env = &templates.Env{
				Facts: map[string]any{
					"hostname": "testhost",
				},
				Data: map[string]any{},
			}
		})

		It("Should write files to the target directory", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("hello world"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Scaffold(ctx, env, prop, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Changed).To(ContainElement(filepath.Join(targetDir, "config.txt")))

			content, err := os.ReadFile(filepath.Join(targetDir, "config.txt"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("hello world"))
		})

		It("Should expand templates when writing files", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("host: [[ facts.hostname ]]"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineJet,
				LeftDelimiter:  "[[",
				RightDelimiter: "]]",
			}

			_, err = provider.Scaffold(ctx, env, prop, false)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filepath.Join(targetDir, "config.txt"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("host: testhost"))
		})

		It("Should not write files in noop mode", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			state, err := provider.Scaffold(ctx, env, prop, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Changed).To(ContainElement(filepath.Join(targetDir, "config.txt")))

			_, err = os.Stat(filepath.Join(targetDir, "config.txt"))
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("Should create nested directories when writing", func(ctx context.Context) {
			nestedDir := filepath.Join(sourceDir, "sub", "dir")
			err := os.MkdirAll(nestedDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(nestedDir, "file.txt"), []byte("nested"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			_, err = provider.Scaffold(ctx, env, prop, false)
			Expect(err).ToNot(HaveOccurred())

			content, err := os.ReadFile(filepath.Join(targetDir, "sub", "dir", "file.txt"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(Equal("nested"))
		})

		It("Should report stable files after second scaffold", func(ctx context.Context) {
			err := os.WriteFile(filepath.Join(sourceDir, "config.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   targetDir,
					Ensure: model.EnsurePresent,
				},
				Source:         sourceDir,
				Engine:         model.ScaffoldEngineGo,
				LeftDelimiter:  "{{",
				RightDelimiter: "}}",
			}

			// First scaffold writes the file
			state, err := provider.Scaffold(ctx, env, prop, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Changed).To(HaveLen(1))

			// Second scaffold reports file as stable
			state, err = provider.Scaffold(ctx, env, prop, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Stable).To(ContainElement(filepath.Join(targetDir, "config.txt")))
			Expect(state.Metadata.Changed).To(BeEmpty())
		})
	})

	Describe("Remove", func() {
		var (
			tmpDir    string
			targetDir string
		)

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
			targetDir = filepath.Join(tmpDir, "target")

			err := os.MkdirAll(targetDir, 0755)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should remove files listed in metadata", func(ctx context.Context) {
			file1 := filepath.Join(targetDir, "file1.txt")
			file2 := filepath.Join(targetDir, "file2.txt")
			err := os.WriteFile(file1, []byte("content1"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(file2, []byte("content2"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Changed: []string{file1},
					Stable:  []string{file2},
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(file1).ToNot(BeAnExistingFile())
			Expect(file2).ToNot(BeAnExistingFile())
			Expect(targetDir).ToNot(BeADirectory())
		})

		It("Should not remove purged files", func(ctx context.Context) {
			managed := filepath.Join(targetDir, "managed.txt")
			purged := filepath.Join(targetDir, "purged.txt")
			err := os.WriteFile(managed, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(purged, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Stable: []string{managed},
					Purged: []string{purged},
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(managed).ToNot(BeAnExistingFile())
			Expect(purged).To(BeAnExistingFile())
			Expect(targetDir).To(BeADirectory())
		})

		It("Should remove empty parent directories", func(ctx context.Context) {
			subDir := filepath.Join(targetDir, "sub")
			err := os.MkdirAll(subDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			file := filepath.Join(subDir, "file.txt")
			err = os.WriteFile(file, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Stable: []string{file},
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(file).ToNot(BeAnExistingFile())
			Expect(subDir).ToNot(BeADirectory())
			Expect(targetDir).ToNot(BeADirectory())
		})

		It("Should remove deeply nested empty directories", func(ctx context.Context) {
			deepDir := filepath.Join(targetDir, "a", "b", "c")
			err := os.MkdirAll(deepDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			file := filepath.Join(deepDir, "file.txt")
			err = os.WriteFile(file, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Stable: []string{file},
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(file).ToNot(BeAnExistingFile())
			Expect(filepath.Join(targetDir, "a", "b", "c")).ToNot(BeADirectory())
			Expect(filepath.Join(targetDir, "a", "b")).ToNot(BeADirectory())
			Expect(filepath.Join(targetDir, "a")).ToNot(BeADirectory())
			Expect(targetDir).ToNot(BeADirectory())
		})

		It("Should remove the target directory itself", func(ctx context.Context) {
			file := filepath.Join(targetDir, "file.txt")
			err := os.WriteFile(file, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Stable: []string{file},
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(file).ToNot(BeAnExistingFile())
			Expect(targetDir).ToNot(BeADirectory())
		})

		It("Should not remove target directory when purged files exist", func(ctx context.Context) {
			file1 := filepath.Join(targetDir, "remove.txt")
			file2 := filepath.Join(targetDir, "keep.txt")
			err := os.WriteFile(file1, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(file2, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Stable: []string{file1},
					Purged: []string{file2},
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(file1).ToNot(BeAnExistingFile())
			Expect(file2).To(BeAnExistingFile())
			Expect(targetDir).To(BeADirectory())
		})

		It("Should handle already removed files without error", func(ctx context.Context) {
			file := filepath.Join(targetDir, "gone.txt")

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Changed: []string{file},
				},
			}

			err := provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should return error for non-absolute paths", func(ctx context.Context) {
			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Stable: []string{"relative/path.txt"},
				},
			}

			err := provider.Remove(ctx, prop, state)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not an absolute path"))
		})

		It("Should handle empty metadata lists", func(ctx context.Context) {
			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{},
			}

			err := provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(targetDir).ToNot(BeADirectory())
		})

		It("Should not mutate the state metadata slices", func(ctx context.Context) {
			file1 := filepath.Join(targetDir, "changed.txt")
			file2 := filepath.Join(targetDir, "stable.txt")
			err := os.WriteFile(file1, []byte("c"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(file2, []byte("s"), 0644)
			Expect(err).ToNot(HaveOccurred())

			prop := &model.ScaffoldResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: targetDir,
				},
			}

			changed := []string{file1}
			stable := []string{file2}

			state := &model.ScaffoldState{
				Metadata: &model.ScaffoldMetadata{
					Changed: changed,
					Stable:  stable,
				},
			}

			err = provider.Remove(ctx, prop, state)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Metadata.Changed).To(Equal([]string{file1}))
			Expect(state.Metadata.Stable).To(Equal([]string{file2}))
			Expect(targetDir).ToNot(BeADirectory())
		})
	})
})
