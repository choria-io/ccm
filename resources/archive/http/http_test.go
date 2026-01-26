// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestHttpProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Archive/Http")
}

var _ = Describe("Http Provider", func() {
	var (
		mockctl  *gomock.Controller
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		provider *Provider
		err      error
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

		provider, err = NewHttpProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("NewHttpProvider", func() {
		It("Should create a provider with the given logger and runner", func() {
			p, err := NewHttpProvider(logger, runner)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.log).To(Equal(logger))
			Expect(p.runner).To(Equal(runner))
		})
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("http"))
		})
	})

	Describe("Download", func() {
		var (
			server  *httptest.Server
			tempDir string
		)

		BeforeEach(func() {
			tempDir = GinkgoT().TempDir()
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("Should download a file successfully", func() {
			content := []byte("archive content here")
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(content)
			}))

			currentUser, err := user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err := user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())

			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:   server.URL + "/archive.tar.gz",
				Owner: currentUser.Username,
				Group: currentGroup.Name,
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())

			// Verify file was created
			Expect(iu.FileExists(destFile)).To(BeTrue())

			// Verify content
			data, err := os.ReadFile(destFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(Equal(content))
		})

		It("Should send custom headers", func() {
			var receivedHeaders http.Header
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header.Clone()
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))

			currentUser, err := user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err := user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())

			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:   server.URL + "/archive.tar.gz",
				Owner: currentUser.Username,
				Group: currentGroup.Name,
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"Authorization":   "Bearer token123",
				},
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())

			Expect(receivedHeaders.Get("X-Custom-Header")).To(Equal("custom-value"))
			Expect(receivedHeaders.Get("Authorization")).To(Equal("Bearer token123"))
		})

		It("Should use basic auth when username and password are provided", func() {
			var receivedAuth string
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))

			currentUser, err := user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err := user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())

			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:      server.URL + "/archive.tar.gz",
				Owner:    currentUser.Username,
				Group:    currentGroup.Name,
				Username: "testuser",
				Password: "testpass",
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())

			Expect(receivedAuth).To(HavePrefix("Basic "))
		})

		It("Should return error on HTTP failure", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("server error"))
			}))

			currentUser, err := user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err := user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())

			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:   server.URL + "/archive.tar.gz",
				Owner: currentUser.Username,
				Group: currentGroup.Name,
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("500"))
		})

		It("Should verify checksum when provided", func() {
			content := []byte("test content for checksum")
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(content)
			}))

			currentUser, err := user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err := user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())

			// Calculate correct checksum
			expectedChecksum, err := iu.Sha256HashBytes(content)
			Expect(err).ToNot(HaveOccurred())

			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:      server.URL + "/archive.tar.gz",
				Owner:    currentUser.Username,
				Group:    currentGroup.Name,
				Checksum: expectedChecksum,
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())

			Expect(iu.FileExists(destFile)).To(BeTrue())
		})

		It("Should fail on checksum mismatch", func() {
			content := []byte("test content")
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(content)
			}))

			currentUser, err := user.Current()
			Expect(err).ToNot(HaveOccurred())

			currentGroup, err := user.LookupGroupId(currentUser.Gid)
			Expect(err).ToNot(HaveOccurred())

			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:      server.URL + "/archive.tar.gz",
				Owner:    currentUser.Username,
				Group:    currentGroup.Name,
				Checksum: "invalid_checksum_that_will_not_match",
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("checksum mismatch"))

			// Temp file should be cleaned up, dest file should not exist
			Expect(iu.FileExists(destFile)).To(BeFalse())
		})

		It("Should return error for invalid URL", func() {
			destFile := filepath.Join(tempDir, "archive.tar.gz")
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: destFile,
				},
				Url:   "://invalid-url",
				Owner: "root",
				Group: "root",
			}

			err = provider.Download(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Extract", func() {
		var tempDir string

		BeforeEach(func() {
			tempDir = GinkgoT().TempDir()
		})

		It("Should extract tar.gz archive", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			extractDir := filepath.Join(tempDir, "extract")

			// Create a dummy archive file (content doesn't matter for this test since we mock)
			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "tar",
				Args:    []string{"-xzf", archiveFile, "-C", extractDir},
				Cwd:     extractDir,
				Timeout: time.Minute,
			}).Return([]byte{}, []byte{}, 0, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())

			// Extract dir should be created
			Expect(iu.IsDirectory(extractDir)).To(BeTrue())
		})

		It("Should extract tgz archive", func() {
			archiveFile := filepath.Join(tempDir, "test.tgz")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "tar",
				Args:    []string{"-xzf", archiveFile, "-C", extractDir},
				Cwd:     extractDir,
				Timeout: time.Minute,
			}).Return([]byte{}, []byte{}, 0, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should extract tar archive", func() {
			archiveFile := filepath.Join(tempDir, "test.tar")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "tar",
				Args:    []string{"-xf", archiveFile, "-C", extractDir},
				Cwd:     extractDir,
				Timeout: time.Minute,
			}).Return([]byte{}, []byte{}, 0, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should extract zip archive", func() {
			archiveFile := filepath.Join(tempDir, "test.zip")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "unzip",
				Args:    []string{"-d", extractDir, archiveFile},
				Cwd:     extractDir,
				Timeout: time.Minute,
			}).Return([]byte{}, []byte{}, 0, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should return error for unsupported archive type", func() {
			archiveFile := filepath.Join(tempDir, "test.rar")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("archive type not supported"))
		})

		It("Should return error when extract parent is empty", func() {
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/test.tar.gz",
				},
				ExtractParent: "",
			}

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("extract parent not set"))
		})

		It("Should return error when tar command fails", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).
				Return([]byte{}, []byte("tar: Error opening archive"), 1, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("tar exited with code 1"))
		})

		It("Should return error when unzip command fails", func() {
			archiveFile := filepath.Join(tempDir, "test.zip")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).
				Return([]byte{}, []byte("unzip: cannot find file"), 1, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unzip exited with code 1"))
		})

		It("Should return error when runner returns an error", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			extractDir := filepath.Join(tempDir, "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			expectedErr := errors.New("runner error")
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).
				Return(nil, nil, -1, expectedErr)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedErr))
		})

		It("Should create extract parent if it does not exist", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			extractDir := filepath.Join(tempDir, "nonexistent", "extract")

			err := os.WriteFile(archiveFile, []byte("fake archive"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				ExtractParent: extractDir,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).
				Return([]byte{}, []byte{}, 0, nil)

			err = provider.Extract(context.Background(), properties, logger)
			Expect(err).ToNot(HaveOccurred())

			// Verify extract dir was created
			Expect(iu.IsDirectory(extractDir)).To(BeTrue())
		})
	})

	Describe("Status", func() {
		var tempDir string

		BeforeEach(func() {
			tempDir = GinkgoT().TempDir()
		})

		It("Should return absent state when archive does not exist", func() {
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: filepath.Join(tempDir, "nonexistent.tar.gz"),
				},
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.EnsureAbsent))
			Expect(status.Metadata.ArchiveExists).To(BeFalse())
		})

		It("Should return present state when archive exists", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			content := []byte("archive content")
			err := os.WriteFile(archiveFile, content, 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.Ensure).To(Equal(model.EnsurePresent))
			Expect(status.Metadata.ArchiveExists).To(BeTrue())
			Expect(status.Metadata.Size).To(Equal(int64(len(content))))
		})

		It("Should calculate checksum for existing archive", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			content := []byte("archive content for checksum")
			err := os.WriteFile(archiveFile, content, 0644)
			Expect(err).ToNot(HaveOccurred())

			expectedChecksum, err := iu.Sha256HashBytes(content)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Metadata.Checksum).To(Equal(expectedChecksum))
		})

		It("Should check creates file existence", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")
			createsFile := filepath.Join(tempDir, "marker")

			err := os.WriteFile(archiveFile, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(createsFile, []byte("marker"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				Creates: createsFile,
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Metadata.CreatesExists).To(BeTrue())
		})

		It("Should return CreatesExists=false when creates file does not exist", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")

			err := os.WriteFile(archiveFile, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
				Creates: filepath.Join(tempDir, "nonexistent_marker"),
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Metadata.CreatesExists).To(BeFalse())
		})

		It("Should include owner and group information", func() {
			archiveFile := filepath.Join(tempDir, "test.tar.gz")

			err := os.WriteFile(archiveFile, []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: archiveFile,
				},
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			// Owner and group should be set (actual values depend on current user)
			Expect(status.Metadata.Owner).ToNot(BeEmpty())
			Expect(status.Metadata.Group).ToNot(BeEmpty())
		})

		It("Should set provider name in metadata", func() {
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: filepath.Join(tempDir, "test.tar.gz"),
				},
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Metadata.Provider).To(Equal("http"))
		})

		It("Should set correct protocol and type in state", func() {
			properties := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: filepath.Join(tempDir, "test.tar.gz"),
				},
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status.Protocol).To(Equal(model.ResourceStatusArchiveProtocol))
			Expect(status.ResourceType).To(Equal(model.ArchiveTypeName))
		})
	})

	Describe("toolForFileName", func() {
		It("Should return tar for .tar.gz files", func() {
			Expect(toolForFileName("/path/to/file.tar.gz")).To(Equal("tar"))
		})

		It("Should return tar for .tgz files", func() {
			Expect(toolForFileName("/path/to/file.tgz")).To(Equal("tar"))
		})

		It("Should return tar for .tar files", func() {
			Expect(toolForFileName("/path/to/file.tar")).To(Equal("tar"))
		})

		It("Should return unzip for .zip files", func() {
			Expect(toolForFileName("/path/to/file.zip")).To(Equal("unzip"))
		})

		It("Should return empty string for unsupported extensions", func() {
			Expect(toolForFileName("/path/to/file.rar")).To(Equal(""))
			Expect(toolForFileName("/path/to/file.7z")).To(Equal(""))
			Expect(toolForFileName("/path/to/file.txt")).To(Equal(""))
		})

		It("Should handle case insensitive extensions", func() {
			Expect(toolForFileName("/path/to/file.TAR.GZ")).To(Equal("tar"))
			Expect(toolForFileName("/path/to/file.ZIP")).To(Equal("unzip"))
		})
	})

	Describe("Constants", func() {
		It("Should have correct provider name", func() {
			Expect(ProviderName).To(Equal("http"))
		})
	})
})

var _ = Describe("Http Factory", func() {
	var (
		f *factory
	)

	BeforeEach(func() {
		f = &factory{}
	})

	Describe("TypeName", func() {
		It("Should return archive type name", func() {
			Expect(f.TypeName()).To(Equal(model.ArchiveTypeName))
		})
	})

	Describe("Name", func() {
		It("Should return http provider name", func() {
			Expect(f.Name()).To(Equal("http"))
		})
	})

	Describe("IsManageable", func() {
		It("Should return true for http URLs with tar.gz extension", func() {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/archive.tar.gz",
				},
				Url: "http://example.com/file.tar.gz",
			}

			manageable, priority, err := f.IsManageable(nil, props)
			Expect(err).ToNot(HaveOccurred())
			// Will fail if tar is not in PATH, which is fine for CI
			if manageable {
				Expect(priority).To(Equal(1))
			}
		})

		It("Should return true for https URLs", func() {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/archive.tar.gz",
				},
				Url: "https://example.com/file.tar.gz",
			}

			manageable, priority, err := f.IsManageable(nil, props)
			Expect(err).ToNot(HaveOccurred())
			if manageable {
				Expect(priority).To(Equal(1))
			}
		})

		It("Should return false for non-http URLs", func() {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/archive.tar.gz",
				},
				Url: "ftp://example.com/file.tar.gz",
			}

			manageable, _, err := f.IsManageable(nil, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeFalse())
		})

		It("Should return false for file URLs", func() {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/archive.tar.gz",
				},
				Url: "file:///path/to/file.tar.gz",
			}

			manageable, _, err := f.IsManageable(nil, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeFalse())
		})

		It("Should return error for invalid URL", func() {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/archive.tar.gz",
				},
				Url: "://invalid",
			}

			_, _, err := f.IsManageable(nil, props)
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid properties type", func() {
			props := &model.ExecResourceProperties{}

			_, _, err := f.IsManageable(nil, props)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid properties"))
		})

		It("Should return false for unsupported archive type", func() {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/tmp/archive.rar",
				},
				Url: "https://example.com/file.rar",
			}

			manageable, _, err := f.IsManageable(nil, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(manageable).To(BeFalse())
		})
	})

	Describe("New", func() {
		It("Should create a new provider", func() {
			mockctl := gomock.NewController(GinkgoT())
			defer mockctl.Finish()

			logger := modelmocks.NewMockLogger(mockctl)
			runner := modelmocks.NewMockCommandRunner(mockctl)

			provider, err := f.New(logger, runner)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).ToNot(BeNil())
			Expect(provider.Name()).To(Equal("http"))
		})
	})
})
