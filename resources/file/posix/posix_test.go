// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestPosixProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/File/Posix")
}

var _ = Describe("Posix Provider", func() {
	var (
		mockctl  *gomock.Controller
		logger   *modelmocks.MockLogger
		provider *Provider
		err      error
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

		provider, err = NewPosixProvider(logger)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("Status", func() {
		It("Should return correct metadata for an existing file", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "testfile.txt")
			content := []byte("test content for checksum")

			err := os.WriteFile(testFile, content, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Calculate expected checksum
			hasher := sha256.New()
			hasher.Write(content)
			expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

			res, err := provider.Status(context.Background(), testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())

			Expect(res.Ensure).To(Equal(model.EnsurePresent))
			Expect(res.Metadata.Name).To(Equal(testFile))
			Expect(res.Metadata.Provider).To(Equal(ProviderName))
			Expect(res.Metadata.Checksum).To(Equal(expectedChecksum))
			// Owner and Group should be populated (actual values depend on system)
			Expect(res.Metadata.Owner).ToNot(BeEmpty())
			Expect(res.Metadata.Group).ToNot(BeEmpty())
		})

		It("Should return absent for a non-existent file", func() {
			nonExistentFile := "/tmp/this-file-should-not-exist-12345"

			res, err := provider.Status(context.Background(), nonExistentFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())

			Expect(res.Ensure).To(Equal(model.EnsureAbsent))
			Expect(res.Metadata.Name).To(Equal(nonExistentFile))
			Expect(res.Metadata.Provider).To(Equal(ProviderName))
		})

		It("Should return correct mode in octal format", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "testfile_mode.txt")
			content := []byte("mode test")

			err := os.WriteFile(testFile, content, 0755)
			Expect(err).ToNot(HaveOccurred())

			res, err := provider.Status(context.Background(), testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())

			// Mode should contain the permission bits (last 3 octal digits should be 755)
			Expect(res.Metadata.Mode).To(ContainSubstring("755"))
		})
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("posix"))
		})
	})

	Describe("Store", func() {
		var (
			currentUser  *user.User
			currentGroup *user.Group
		)

		BeforeEach(func() {
			var err error
			currentUser, err = user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err = user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("storing files with different modes",
			func(mode string, expectedPerm os.FileMode) {
				tmpDir := GinkgoT().TempDir()
				testFile := filepath.Join(tmpDir, "modefile.txt")
				content := []byte("mode test")

				err := provider.Store(context.Background(), testFile, content, "", currentUser.Username, currentGroup.Name, mode)
				Expect(err).ToNot(HaveOccurred())

				stat, err := os.Stat(testFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(stat.Mode().Perm()).To(Equal(expectedPerm))
			},
			Entry("mode 0644", "0644", os.FileMode(0644)),
			Entry("mode 0755", "0755", os.FileMode(0755)),
			Entry("mode 0600", "0600", os.FileMode(0600)),
			Entry("mode 0640", "0640", os.FileMode(0640)),
		)

		DescribeTable("error cases",
			func(setupPath func() string, owner, group, mode, errContains string) {
				testFile := setupPath()
				content := []byte("test")

				currentOwner := owner
				if owner == "" {
					currentOwner = currentUser.Username
				}
				currentGrp := group
				if group == "" {
					currentGrp = currentGroup.Name
				}

				err := provider.Store(context.Background(), testFile, content, "", currentOwner, currentGrp, mode)
				Expect(err).To(HaveOccurred())
				if errContains != "" {
					Expect(err.Error()).To(ContainSubstring(errContains))
				}
			},
			Entry("non-existent directory",
				func() string { return "/nonexistent/directory/file.txt" },
				"", "", "0644", "is not a directory"),
			Entry("invalid mode",
				func() string { return filepath.Join(GinkgoT().TempDir(), "badmode.txt") },
				"", "", "invalid", ""),
			Entry("invalid owner",
				func() string { return filepath.Join(GinkgoT().TempDir(), "badowner.txt") },
				"nonexistent_user_12345", "", "0644", "could not lookup user"),
			Entry("invalid group",
				func() string { return filepath.Join(GinkgoT().TempDir(), "badgroup.txt") },
				"", "nonexistent_group_12345", "0644", "could not lookup group"),
		)

		It("Should store a file with correct content", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "newfile.txt")
			content := []byte("hello world")

			err := provider.Store(context.Background(), testFile, content, "", currentUser.Username, currentGroup.Name, "0644")
			Expect(err).ToNot(HaveOccurred())

			readContent, err := os.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(readContent).To(Equal(content))
		})

		It("Should overwrite an existing file", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "existingfile.txt")

			err := os.WriteFile(testFile, []byte("original content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			newContent := []byte("new content")
			err = provider.Store(context.Background(), testFile, newContent, "", currentUser.Username, currentGroup.Name, "0644")
			Expect(err).ToNot(HaveOccurred())

			readContent, err := os.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(readContent).To(Equal(newContent))
		})

		It("Should store empty content", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "emptyfile.txt")

			err := provider.Store(context.Background(), testFile, []byte{}, "", currentUser.Username, currentGroup.Name, "0644")
			Expect(err).ToNot(HaveOccurred())

			stat, err := os.Stat(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Size()).To(Equal(int64(0)))
		})

		It("Should verify stored file matches Status output", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "verifyfile.txt")
			content := []byte("verify content")

			err := provider.Store(context.Background(), testFile, content, "", currentUser.Username, currentGroup.Name, "0640")
			Expect(err).ToNot(HaveOccurred())

			status, err := provider.Status(context.Background(), testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Ensure).To(Equal(model.EnsurePresent))
			Expect(status.Metadata.Owner).To(Equal(currentUser.Username))
			Expect(status.Metadata.Group).To(Equal(currentGroup.Name))
			Expect(status.Metadata.Mode).To(Equal("0640"))

			hasher := sha256.New()
			hasher.Write(content)
			expectedChecksum := hex.EncodeToString(hasher.Sum(nil))
			Expect(status.Metadata.Checksum).To(Equal(expectedChecksum))
		})

		It("Should copy from source file when source is specified", func() {
			tmpDir := GinkgoT().TempDir()
			sourceFile := filepath.Join(tmpDir, "source.txt")
			destFile := filepath.Join(tmpDir, "dest.txt")
			sourceContent := []byte("content from source file")

			err := os.WriteFile(sourceFile, sourceContent, 0644)
			Expect(err).ToNot(HaveOccurred())

			err = provider.Store(context.Background(), destFile, nil, sourceFile, currentUser.Username, currentGroup.Name, "0644")
			Expect(err).ToNot(HaveOccurred())

			readContent, err := os.ReadFile(destFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(readContent).To(Equal(sourceContent))
		})

		It("Should fail when source file does not exist", func() {
			tmpDir := GinkgoT().TempDir()
			destFile := filepath.Join(tmpDir, "dest.txt")
			nonExistentSource := filepath.Join(tmpDir, "nonexistent.txt")

			err := provider.Store(context.Background(), destFile, nil, nonExistentSource, currentUser.Username, currentGroup.Name, "0644")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})

		It("Should prefer source over content when both are specified", func() {
			tmpDir := GinkgoT().TempDir()
			sourceFile := filepath.Join(tmpDir, "source.txt")
			destFile := filepath.Join(tmpDir, "dest.txt")
			sourceContent := []byte("content from source")
			inlineContent := []byte("inline content")

			err := os.WriteFile(sourceFile, sourceContent, 0644)
			Expect(err).ToNot(HaveOccurred())

			err = provider.Store(context.Background(), destFile, inlineContent, sourceFile, currentUser.Username, currentGroup.Name, "0644")
			Expect(err).ToNot(HaveOccurred())

			readContent, err := os.ReadFile(destFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(readContent).To(Equal(sourceContent))
		})
	})

	Describe("NewPosixProvider", func() {
		It("Should create a provider with the given logger", func() {
			p, err := NewPosixProvider(logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.log).To(Equal(logger))
		})
	})

	Describe("CreateDirectory", func() {
		var (
			currentUser  *user.User
			currentGroup *user.Group
		)

		BeforeEach(func() {
			var err error
			currentUser, err = user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err = user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should create a directory with correct permissions", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "newdir")

			err := provider.CreateDirectory(context.Background(), testDir, currentUser.Username, currentGroup.Name, "0755")
			Expect(err).ToNot(HaveOccurred())

			stat, err := os.Stat(testDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.IsDir()).To(BeTrue())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
		})

		It("Should create nested directories", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "parent", "child", "grandchild")

			err := provider.CreateDirectory(context.Background(), testDir, currentUser.Username, currentGroup.Name, "0750")
			Expect(err).ToNot(HaveOccurred())

			stat, err := os.Stat(testDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.IsDir()).To(BeTrue())
		})

		It("Should not fail when directory already exists", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "existingdir")

			err := os.Mkdir(testDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			err = provider.CreateDirectory(context.Background(), testDir, currentUser.Username, currentGroup.Name, "0755")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should remediate mode on existing directory", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "existingdir")

			// Create directory and set wrong mode explicitly (umask affects Mkdir)
			err := os.Mkdir(testDir, 0755)
			Expect(err).ToNot(HaveOccurred())
			err = os.Chmod(testDir, 0777)
			Expect(err).ToNot(HaveOccurred())

			// Verify initial mode
			stat, err := os.Stat(testDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0777)))

			// CreateDirectory should fix the mode
			err = provider.CreateDirectory(context.Background(), testDir, currentUser.Username, currentGroup.Name, "0750")
			Expect(err).ToNot(HaveOccurred())

			// Verify mode was corrected
			stat, err = os.Stat(testDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0750)))
		})

		It("Should fail with invalid owner", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "badowner")

			err := provider.CreateDirectory(context.Background(), testDir, "nonexistent_user_12345", currentGroup.Name, "0755")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not lookup user"))
		})

		It("Should fail with invalid group", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "badgroup")

			err := provider.CreateDirectory(context.Background(), testDir, currentUser.Username, "nonexistent_group_12345", "0755")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not lookup group"))
		})

		It("Should fail with invalid mode", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "badmode")

			err := provider.CreateDirectory(context.Background(), testDir, currentUser.Username, currentGroup.Name, "invalid")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Status for directories", func() {
		It("Should return directory ensure for existing directories", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "testdir")

			err := os.Mkdir(testDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			res, err := provider.Status(context.Background(), testDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Ensure).To(Equal(model.FileEnsureDirectory))
		})

		It("Should not calculate checksum for directories", func() {
			tmpDir := GinkgoT().TempDir()
			testDir := filepath.Join(tmpDir, "testdir")

			err := os.Mkdir(testDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			res, err := provider.Status(context.Background(), testDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Metadata.Checksum).To(BeEmpty())
		})

		It("Should return absent for non-existent directory", func() {
			nonExistentDir := "/tmp/this-directory-should-not-exist-12345"

			res, err := provider.Status(context.Background(), nonExistentDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Ensure).To(Equal(model.EnsureAbsent))
		})

		It("Should return present for files (not directory)", func() {
			tmpDir := GinkgoT().TempDir()
			testFile := filepath.Join(tmpDir, "testfile.txt")

			err := os.WriteFile(testFile, []byte("test content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			res, err := provider.Status(context.Background(), testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.Ensure).To(Equal(model.EnsurePresent))
		})
	})
})
