// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestApply(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Apply")
}

var _ = Describe("Apply", func() {
	var (
		mockctl *gomock.Controller
		logger  *modelmocks.MockLogger
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("Resources", func() {
		It("Should return the resources list", func() {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.PackageTypeName: &model.PackageResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name: "test",
						}},
					},
				},
			}

			resources := apply.Resources()
			Expect(resources).To(HaveLen(1))
			Expect(resources[0]).To(HaveKey(model.PackageTypeName))
		})

		It("Should return empty list when no resources", func() {
			apply := &Apply{}
			resources := apply.Resources()
			Expect(resources).To(BeEmpty())
		})
	})

	Describe("Data", func() {
		It("Should return the data map", func() {
			data := map[string]any{
				"key": "value",
			}
			apply := &Apply{
				data: data,
			}

			result := apply.Data()
			Expect(result).To(Equal(data))
		})

		It("Should return nil when no data", func() {
			apply := &Apply{}
			result := apply.Data()
			Expect(result).To(BeNil())
		})
	})

	Describe("FailOnError", func() {
		It("Should return true when failOnError is set", func() {
			apply := &Apply{
				failOnError: true,
			}

			Expect(apply.FailOnError()).To(BeTrue())
		})

		It("Should return false when failOnError is not set", func() {
			apply := &Apply{
				failOnError: false,
			}

			Expect(apply.FailOnError()).To(BeFalse())
		})

		It("Should return false by default", func() {
			apply := &Apply{}
			Expect(apply.FailOnError()).To(BeFalse())
		})
	})

	Describe("Thread safety", func() {
		It("Should handle concurrent access to Resources", func() {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.PackageTypeName: &model.PackageResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name: "test",
						}},
					},
				},
			}

			done := make(chan bool)

			// Concurrent reads
			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					resources := apply.Resources()
					Expect(resources).To(HaveLen(1))
					done <- true
				}()
			}

			// Wait for all goroutines
			for i := 0; i < 10; i++ {
				<-done
			}
		})

		It("Should handle concurrent access to Data", func() {
			apply := &Apply{
				data: map[string]any{"key": "value"},
			}

			done := make(chan bool)

			// Concurrent reads
			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					data := apply.Data()
					Expect(data).To(HaveKey("key"))
					done <- true
				}()
			}

			// Wait for all goroutines
			for i := 0; i < 10; i++ {
				<-done
			}
		})

		It("Should handle concurrent access to FailOnError", func() {
			apply := &Apply{
				failOnError: true,
			}

			done := make(chan bool)

			// Concurrent reads
			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					ff := apply.FailOnError()
					Expect(ff).To(BeTrue())
					done <- true
				}()
			}

			// Wait for all goroutines
			for i := 0; i < 10; i++ {
				<-done
			}
		})
	})

	Describe("Execute", func() {
		var (
			facts      map[string]any
			data       map[string]any
			mgr        *modelmocks.MockManager
			mgrLogger  *modelmocks.MockLogger
			userLogger *modelmocks.MockLogger
			runner     *modelmocks.MockCommandRunner
			session    *modelmocks.MockSessionStore
		)

		BeforeEach(func() {
			facts = make(map[string]any)
			data = make(map[string]any)
			mgr, mgrLogger = modelmocks.NewManager(facts, data, false, mockctl)
			userLogger = modelmocks.NewMockLogger(mockctl)
			runner = modelmocks.NewMockCommandRunner(mockctl)
			session = modelmocks.NewMockSessionStore(mockctl)

			mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
			userLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

			registry.Clear()
		})

		It("Should fail when StartSession fails", func(ctx context.Context) {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{},
			}

			mgr.EXPECT().StartSession(apply).Return(nil, fmt.Errorf("session failed"))

			result, err := apply.Execute(ctx, mgr, false, userLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session failed"))
			Expect(result).To(BeNil())
		})

		It("Should return session when no resources", func(ctx context.Context) {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{},
			}

			mgr.EXPECT().StartSession(apply).Return(session, nil)

			result, err := apply.Execute(ctx, mgr, false, userLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(session))
		})

		It("Should fail when no provider is available for package", func(ctx context.Context) {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.PackageTypeName: &model.PackageResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name:     "vim",
							Ensure:   "present",
							Provider: "nonexistent",
						},
					}},
				},
			}

			mgr.EXPECT().StartSession(apply).Return(session, nil)
			mgrLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

			result, err := apply.Execute(ctx, mgr, false, userLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no suitable provider found"))
			Expect(result).To(BeNil())
		})

		It("Should fail when no provider is available for service", func(ctx context.Context) {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.ServiceTypeName: &model.ServiceResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name:     "nginx",
							Ensure:   "running",
							Provider: "nonexistent",
						},
					}},
				},
			}

			mgr.EXPECT().StartSession(apply).Return(session, nil)
			mgrLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
			mgrLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

			result, err := apply.Execute(ctx, mgr, false, userLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no suitable provider found"))
			Expect(result).To(BeNil())
		})

		It("Should fail when no provider is available for file", func(ctx context.Context) {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.FileTypeName: &model.FileResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name:     "/tmp/test",
							Ensure:   "present",
							Provider: "nonexistent",
						},
						Owner:    "root",
						Group:    "root",
						Mode:     "0644",
						Contents: "test content",
					}},
				},
			}

			mgr.EXPECT().StartSession(apply).Return(session, nil)
			mgrLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

			result, err := apply.Execute(ctx, mgr, false, userLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no suitable provider found"))
			Expect(result).To(BeNil())
		})
	})
})

// mockObjectResult implements jetstream.ObjectResult for testing
type mockObjectResult struct {
	io.Reader
	info   *jetstream.ObjectInfo
	closed bool
}

func (m *mockObjectResult) Info() (*jetstream.ObjectInfo, error) {
	return m.info, nil
}

func (m *mockObjectResult) Close() error {
	m.closed = true
	return nil
}

func (m *mockObjectResult) Error() error {
	return nil
}

// createTarGz creates a tar.gz archive containing a manifest.yaml file
func createTarGz(manifestContent string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add manifest.yaml
	hdr := &tar.Header{
		Name: "manifest.yaml",
		Mode: 0644,
		Size: int64(len(manifestContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(manifestContent)); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

var _ = Describe("ResolveManifestUrl", func() {
	var (
		mockctl *gomock.Controller
		mockMgr *modelmocks.MockManager
		mockJS  *modelmocks.MockJetStream
		mockLog *modelmocks.MockLogger
		ctx     context.Context
		facts   map[string]any
		data    map[string]any
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		facts = map[string]any{"os": "linux"}
		data = make(map[string]any)
		mockMgr, mockLog = modelmocks.NewManager(facts, data, false, mockctl)
		mockJS = modelmocks.NewMockJetStream(mockctl)
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockMgr.EXPECT().SetData(gomock.Any()).AnyTimes().Return(data)
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	It("returns an error when source is empty", func() {
		_, _, _, err := ResolveManifestUrl(ctx, mockMgr, "", mockLog)
		Expect(err).To(MatchError("source is required"))
	})

	It("returns an error for unsupported URL scheme", func() {
		_, _, _, err := ResolveManifestUrl(ctx, mockMgr, "http://example.com/manifest", mockLog)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported manifest source"))
	})

	It("resolves paths without scheme as files", func() {
		tempDir, err := os.MkdirTemp("", "manifest-test-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tempDir)

		manifestContent := `
data:
  setting: value
ccm:
  resources:
    - package:
        name: vim
        ensure: present
`
		manifestPath := tempDir + "/manifest.yaml"
		err = os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		res, apply, wd, err := ResolveManifestUrl(ctx, mockMgr, manifestPath, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		Expect(wd).To(BeEmpty())
	})

	It("resolves obj:// URLs", func() {
		mockObjStore := modelmocks.NewMockObjectStore(mockctl)

		manifestContent := `
data:
  setting: value
ccm:
  resources:
    - package:
        name: vim
        ensure: present
`
		tarGz, err := createTarGz(manifestContent)
		Expect(err).NotTo(HaveOccurred())

		mockResult := &mockObjectResult{
			Reader: tarGz,
			info:   &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "test.tar.gz"}},
		}

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(ctx, "mybucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(ctx, "manifest.tar.gz").Return(mockResult, nil)

		res, apply, wd, err := ResolveManifestUrl(ctx, mockMgr, "obj://mybucket/manifest.tar.gz", mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		Expect(wd).NotTo(BeEmpty())

		// Cleanup the temp directory
		os.RemoveAll(wd)
	})
})

var _ = Describe("ResolveManifestObjectValue", func() {
	var (
		mockctl      *gomock.Controller
		mockMgr      *modelmocks.MockManager
		mockJS       *modelmocks.MockJetStream
		mockObjStore *modelmocks.MockObjectStore
		mockLog      *modelmocks.MockLogger
		ctx          context.Context
		facts        map[string]any
		data         map[string]any
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		facts = map[string]any{"os": "linux"}
		data = make(map[string]any)
		mockMgr, mockLog = modelmocks.NewManager(facts, data, false, mockctl)
		mockJS = modelmocks.NewMockJetStream(mockctl)
		mockObjStore = modelmocks.NewMockObjectStore(mockctl)
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockMgr.EXPECT().SetData(gomock.Any()).AnyTimes().Return(data)
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	It("returns an error when bucket is empty", func() {
		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "", "key", mockLog)
		Expect(err).To(MatchError("bucket name is required for object store manifest source"))
	})

	It("returns an error when key is empty", func() {
		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "bucket", "", mockLog)
		Expect(err).To(MatchError("key is required for object store manifest source"))
	})

	It("returns an error when JetStream fails", func() {
		jsErr := errors.New("jetstream unavailable")
		mockMgr.EXPECT().JetStream().Return(nil, jsErr)

		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "bucket", "key", mockLog)
		Expect(err).To(MatchError(jsErr))
	})

	It("returns an error when ObjectStore fails", func() {
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(ctx, "missing-bucket").Return(nil, jetstream.ErrBucketNotFound)

		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "missing-bucket", "key", mockLog)
		Expect(err).To(MatchError(jetstream.ErrBucketNotFound))
	})

	It("returns an error when Get fails", func() {
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(ctx, "bucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(ctx, "missing-key").Return(nil, jetstream.ErrObjectNotFound)

		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "bucket", "missing-key", mockLog)
		Expect(err).To(MatchError(jetstream.ErrObjectNotFound))
	})

	It("returns an error when tar.gz is invalid", func() {
		mockResult := &mockObjectResult{
			Reader: bytes.NewReader([]byte("not a tar.gz file")),
			info:   &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "invalid.tar.gz"}},
		}

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(ctx, "bucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(ctx, "invalid.tar.gz").Return(mockResult, nil)

		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "bucket", "invalid.tar.gz", mockLog)
		Expect(err).To(HaveOccurred())
	})

	It("resolves manifest from object store successfully", func() {
		manifestContent := `
data:
  log_level: INFO
ccm:
  resources:
    - package:
        name: nginx
        ensure: present
`
		tarGz, err := createTarGz(manifestContent)
		Expect(err).NotTo(HaveOccurred())

		mockResult := &mockObjectResult{
			Reader: tarGz,
			info:   &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "manifest.tar.gz"}},
		}

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(ctx, "manifests").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(ctx, "app/manifest.tar.gz").Return(mockResult, nil)

		res, apply, wd, err := ResolveManifestObjectValue(ctx, mockMgr, "manifests", "app/manifest.tar.gz", mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		Expect(wd).NotTo(BeEmpty())

		// Verify the resources were parsed
		Expect(apply.Resources()).To(HaveLen(1))

		// Cleanup temp directory
		os.RemoveAll(wd)
	})
})

var _ = Describe("ResolveManifestFilePath", func() {
	var (
		mockctl *gomock.Controller
		mockMgr *modelmocks.MockManager
		mockLog *modelmocks.MockLogger
		ctx     context.Context
		facts   map[string]any
		data    map[string]any
		tempDir string
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		facts = map[string]any{"os": "linux"}
		data = make(map[string]any)
		mockMgr, mockLog = modelmocks.NewManager(facts, data, false, mockctl)
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockMgr.EXPECT().SetData(gomock.Any()).DoAndReturn(func(d map[string]any) map[string]any {
			data = d
			return data
		}).AnyTimes()
		mockMgr.EXPECT().Data().Return(data).AnyTimes()

		var err error
		tempDir, err = os.MkdirTemp("", "manifest-file-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
		os.RemoveAll(tempDir)
	})

	It("returns an error when file does not exist", func() {
		_, _, err := ResolveManifestFilePath(ctx, mockMgr, "/nonexistent/manifest.yaml")
		Expect(err).To(HaveOccurred())
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("resolves manifest from file successfully", func() {
		manifestContent := `
data:
  log_level: DEBUG
ccm:
  resources:
    - package:
        name: vim
        ensure: present
    - package:
        name: git
        ensure: latest
`
		manifestPath := tempDir + "/manifest.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		res, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		Expect(apply.Resources()).To(HaveLen(2))
	})

	It("resolves manifest with hiera data", func() {
		manifestContent := `
hierarchy:
  order:
    - os:{{ lookup('facts.os') }}
    - default
  merge: deep
data:
  log_level: INFO
overrides:
  os:linux:
    log_level: DEBUG
ccm:
  resources:
    - package:
        name: vim
        ensure: present
`
		manifestPath := tempDir + "/manifest.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		res, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		Expect(apply.Resources()).To(HaveLen(1))
	})

	It("resolves manifest with jet templates", func() {
		res, apply, err := ResolveManifestFilePath(ctx, mockMgr, "testdata/jet/manifest.yaml")
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		resources := apply.Resources()
		Expect(resources).To(HaveLen(2))
	})

	It("returns an error for invalid YAML", func() {
		manifestContent := `
data:
  invalid: [unclosed
`
		manifestPath := tempDir + "/invalid.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when manifest has no resources", func() {
		manifestContent := `
data:
  log_level: INFO
ccm:
  other: value
`
		manifestPath := tempDir + "/no-resources.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("manifest must not contain resources"))
	})

	It("parses fail_on_error as true when set", func() {
		manifestContent := `
data:
  log_level: INFO
ccm:
  fail_on_error: true
  resources:
    - package:
        name: vim
        ensure: present
`
		manifestPath := tempDir + "/fail-on-error.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(apply).NotTo(BeNil())
		Expect(apply.FailOnError()).To(BeTrue())
	})

	It("parses fail_on_error as false when set to false", func() {
		manifestContent := `
data:
  log_level: INFO
ccm:
  fail_on_error: false
  resources:
    - package:
        name: vim
        ensure: present
`
		manifestPath := tempDir + "/fail-on-error-false.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(apply).NotTo(BeNil())
		Expect(apply.FailOnError()).To(BeFalse())
	})

	It("defaults fail_on_error to false when not specified", func() {
		manifestContent := `
data:
  log_level: INFO
ccm:
  resources:
    - package:
        name: vim
        ensure: present
`
		manifestPath := tempDir + "/no-fail-on-error.yaml"
		err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
		Expect(err).NotTo(HaveOccurred())

		_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(apply).NotTo(BeNil())
		Expect(apply.FailOnError()).To(BeFalse())
	})
})
