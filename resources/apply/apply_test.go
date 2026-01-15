// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
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
	"net/url"
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

	Describe("String", func() {
		It("Should return a string representation", func() {
			apply := &Apply{
				source: "/path/to/manifest.yaml",
				resources: []map[string]model.ResourceProperties{
					{model.PackageTypeName: &model.PackageResourceProperties{}},
					{model.FileTypeName: &model.FileResourceProperties{}},
				},
			}

			result := apply.String()
			Expect(result).To(Equal("/path/to/manifest.yaml with 2 resources"))
		})

		It("Should handle empty source and resources", func() {
			apply := &Apply{}
			result := apply.String()
			Expect(result).To(Equal(" with 0 resources"))
		})
	})

	Describe("Source", func() {
		It("Should return the source path", func() {
			apply := &Apply{
				source: "/path/to/manifest.yaml",
			}

			Expect(apply.Source()).To(Equal("/path/to/manifest.yaml"))
		})

		It("Should return empty string when no source", func() {
			apply := &Apply{}
			Expect(apply.Source()).To(BeEmpty())
		})
	})

	Describe("MarshalYAML", func() {
		It("Should marshal to YAML correctly", func() {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.PackageTypeName: &model.PackageResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name: "test",
						}},
					},
				},
				data: map[string]any{
					"key": "value",
				},
			}

			yamlBytes, err := apply.MarshalYAML()
			Expect(err).NotTo(HaveOccurred())
			Expect(yamlBytes).NotTo(BeEmpty())
			Expect(string(yamlBytes)).To(ContainSubstring("resources:"))
			Expect(string(yamlBytes)).To(ContainSubstring("data:"))
		})

		It("Should handle empty apply", func() {
			apply := &Apply{}
			yamlBytes, err := apply.MarshalYAML()
			Expect(err).NotTo(HaveOccurred())
			Expect(yamlBytes).NotTo(BeEmpty())
		})
	})

	Describe("MarshalJSON", func() {
		It("Should marshal to JSON correctly", func() {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{
					{model.PackageTypeName: &model.PackageResourceProperties{
						CommonResourceProperties: model.CommonResourceProperties{
							Name: "test",
						}},
					},
				},
				data: map[string]any{
					"key": "value",
				},
			}

			jsonBytes, err := apply.MarshalJSON()
			Expect(err).NotTo(HaveOccurred())
			Expect(jsonBytes).NotTo(BeEmpty())
			Expect(string(jsonBytes)).To(ContainSubstring(`"resources"`))
			Expect(string(jsonBytes)).To(ContainSubstring(`"data"`))
		})

		It("Should handle empty apply", func() {
			apply := &Apply{}
			jsonBytes, err := apply.MarshalJSON()
			Expect(err).NotTo(HaveOccurred())
			Expect(jsonBytes).NotTo(BeEmpty())
		})
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

		It("Should fail when both noop mode and healthCheckOnly are set", func(ctx context.Context) {
			// Create a manager with noop mode enabled
			noopMgr, _ := modelmocks.NewManager(facts, data, true, mockctl)

			apply := &Apply{
				resources: []map[string]model.ResourceProperties{},
			}

			result, err := apply.Execute(ctx, noopMgr, true, userLogger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot set healthceck only and noop mode at the same time"))
			Expect(result).To(BeNil())
		})

		It("Should allow noop mode without healthCheckOnly", func(ctx context.Context) {
			noopMgr, _ := modelmocks.NewManager(facts, data, true, mockctl)
			noopMgr.EXPECT().StartSession(gomock.Any()).Return(session, nil)

			apply := &Apply{
				resources: []map[string]model.ResourceProperties{},
			}

			result, err := apply.Execute(ctx, noopMgr, false, userLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(session))
		})

		It("Should allow healthCheckOnly without noop mode", func(ctx context.Context) {
			apply := &Apply{
				resources: []map[string]model.ResourceProperties{},
			}

			healthcheckLogger := modelmocks.NewMockLogger(mockctl)
			healthcheckLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			userLogger.EXPECT().With("healthcheck", true).Return(healthcheckLogger)
			mgr.EXPECT().StartSession(apply).Return(session, nil)

			result, err := apply.Execute(ctx, mgr, true, userLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(session))
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

		Context("healthcheck logging", func() {
			It("Should add healthcheck attribute to logger when healthCheckOnly is true", func(ctx context.Context) {
				apply := &Apply{
					resources: []map[string]model.ResourceProperties{},
				}

				healthcheckLogger := modelmocks.NewMockLogger(mockctl)
				healthcheckLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

				userLogger.EXPECT().With("healthcheck", true).Return(healthcheckLogger)
				mgr.EXPECT().StartSession(apply).Return(session, nil)

				result, err := apply.Execute(ctx, mgr, true, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(session))
			})

			It("Should not add healthcheck attribute when healthCheckOnly is false", func(ctx context.Context) {
				apply := &Apply{
					resources: []map[string]model.ResourceProperties{},
				}

				// userLogger.With should NOT be called when healthCheckOnly is false
				mgr.EXPECT().StartSession(apply).Return(session, nil)

				result, err := apply.Execute(ctx, mgr, false, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(session))
			})
		})

		Context("fail on error behavior", func() {
			It("Should not terminate on failed resource when healthCheckOnly is true", func(ctx context.Context) {
				apply := &Apply{
					resources:   []map[string]model.ResourceProperties{},
					failOnError: true,
				}

				healthcheckLogger := modelmocks.NewMockLogger(mockctl)
				healthcheckLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

				userLogger.EXPECT().With("healthcheck", true).Return(healthcheckLogger)
				mgr.EXPECT().StartSession(apply).Return(session, nil)

				// When healthCheckOnly is true and failOnError is true,
				// it should NOT log a warning about termination
				result, err := apply.Execute(ctx, mgr, true, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(session))
			})

			It("Should not terminate on failed resource when failOnError is false", func(ctx context.Context) {
				apply := &Apply{
					resources:   []map[string]model.ResourceProperties{},
					failOnError: false,
				}

				mgr.EXPECT().StartSession(apply).Return(session, nil)

				result, err := apply.Execute(ctx, mgr, false, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(session))
			})
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
		_, _, _, err := ResolveManifestUrl(ctx, mockMgr, "ftp://example.com/manifest", mockLog)
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
		mockJS.EXPECT().ObjectStore(gomock.Any(), "mybucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(gomock.Any(), "manifest.tar.gz").Return(mockResult, nil)

		res, apply, wd, err := ResolveManifestUrl(ctx, mockMgr, "obj://mybucket/manifest.tar.gz", mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).NotTo(BeNil())
		Expect(apply).NotTo(BeNil())
		Expect(wd).NotTo(BeEmpty())

		// Cleanup the temp directory
		os.RemoveAll(wd)
	})
})

var _ = Describe("ResolveManifestHttpUrl", func() {
	var (
		mockctl *gomock.Controller
		mockMgr *modelmocks.MockManager
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
		ctx = context.Background()

		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockMgr.EXPECT().SetData(gomock.Any()).AnyTimes().Return(data)
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	It("returns an error when URL is empty", func() {
		_, _, _, err := ResolveManifestHttpUrl(ctx, mockMgr, "", mockLog)
		Expect(err).To(MatchError("URL is required for HTTP manifest source"))
	})

	It("returns an error when URL is invalid", func() {
		_, _, _, err := ResolveManifestHttpUrl(ctx, mockMgr, "://invalid", mockLog)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid URL"))
	})

	It("returns an error when URL scheme is not http or https", func() {
		_, _, _, err := ResolveManifestHttpUrl(ctx, mockMgr, "ftp://example.com/manifest.tar.gz", mockLog)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("URL scheme must be http or https"))
	})

	It("returns an error when HTTP request fails with connection error", func() {
		// Use an invalid host that will fail to connect
		_, _, _, err := ResolveManifestHttpUrl(ctx, mockMgr, "http://localhost:59999/manifest.tar.gz", mockLog)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to fetch manifest from URL"))
	})

	It("returns an error when URL with credentials fails to connect", func() {
		// Use an invalid host with credentials that will fail to connect
		_, _, _, err := ResolveManifestHttpUrl(ctx, mockMgr, "http://user:pass@localhost:59999/manifest.tar.gz", mockLog)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to fetch manifest from URL"))
	})
})

var _ = Describe("redactUrlCredentials", func() {
	It("returns URL unchanged when no credentials present", func() {
		u, _ := url.Parse("https://example.com/path/to/file.tar.gz")
		result := redactUrlCredentials(u)
		Expect(result).To(Equal("https://example.com/path/to/file.tar.gz"))
	})

	It("redacts username and password from URL", func() {
		u, _ := url.Parse("https://myuser:secretpass@example.com/path/to/file.tar.gz")
		result := redactUrlCredentials(u)
		Expect(result).To(Equal("https://%5BREDACTED%5D@example.com/path/to/file.tar.gz"))
		Expect(result).NotTo(ContainSubstring("myuser"))
		Expect(result).NotTo(ContainSubstring("secretpass"))
	})

	It("redacts username-only credentials from URL", func() {
		u, _ := url.Parse("https://myuser@example.com/path/to/file.tar.gz")
		result := redactUrlCredentials(u)
		Expect(result).To(Equal("https://%5BREDACTED%5D@example.com/path/to/file.tar.gz"))
		Expect(result).NotTo(ContainSubstring("myuser"))
	})

	It("preserves query parameters and fragments", func() {
		u, _ := url.Parse("https://user:pass@example.com/path?query=value#fragment")
		result := redactUrlCredentials(u)
		Expect(result).To(Equal("https://%5BREDACTED%5D@example.com/path?query=value#fragment"))
	})

	It("preserves port numbers", func() {
		u, _ := url.Parse("https://user:pass@example.com:8443/path/to/file.tar.gz")
		result := redactUrlCredentials(u)
		Expect(result).To(Equal("https://%5BREDACTED%5D@example.com:8443/path/to/file.tar.gz"))
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
		mockJS.EXPECT().ObjectStore(gomock.Any(), "missing-bucket").Return(nil, jetstream.ErrBucketNotFound)

		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "missing-bucket", "key", mockLog)
		Expect(err).To(MatchError(jetstream.ErrBucketNotFound))
	})

	It("returns an error when Get fails", func() {
		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(gomock.Any(), "bucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(gomock.Any(), "missing-key").Return(nil, jetstream.ErrObjectNotFound)

		_, _, _, err := ResolveManifestObjectValue(ctx, mockMgr, "bucket", "missing-key", mockLog)
		Expect(err).To(MatchError(jetstream.ErrObjectNotFound))
	})

	It("returns an error when tar.gz is invalid", func() {
		mockResult := &mockObjectResult{
			Reader: bytes.NewReader([]byte("not a tar.gz file")),
			info:   &jetstream.ObjectInfo{ObjectMeta: jetstream.ObjectMeta{Name: "invalid.tar.gz"}},
		}

		mockMgr.EXPECT().JetStream().Return(mockJS, nil)
		mockJS.EXPECT().ObjectStore(gomock.Any(), "bucket").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(gomock.Any(), "invalid.tar.gz").Return(mockResult, nil)

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
		mockJS.EXPECT().ObjectStore(gomock.Any(), "manifests").Return(mockObjStore, nil)
		mockObjStore.EXPECT().Get(gomock.Any(), "app/manifest.tar.gz").Return(mockResult, nil)

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
		// Schema validation catches this first - unknown property 'other' and missing resources
		Expect(err.Error()).To(ContainSubstring("jsonschema validation failed"))
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

	Describe("WithOverridingHieraData", func() {
		It("should set the overridingHieraData field", func() {
			apply := &Apply{}
			opt := WithOverridingHieraData("/some/path.yaml")
			err := opt(apply)
			Expect(err).NotTo(HaveOccurred())
			Expect(apply.overridingHieraData).To(Equal("/some/path.yaml"))
		})

		It("should return error for nonexistent hiera file", func() {
			manifestContent := `
data:
  key: value
ccm:
  resources:
    - package:
        name: test
        ensure: present
`
			manifestPath := tempDir + "/invalid-hiera-test.yaml"
			err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = ResolveManifestFilePath(ctx, mockMgr, manifestPath,
				WithOverridingHieraData("/nonexistent/path.yaml"))
			Expect(err).To(HaveOccurred())
		})

		DescribeTable("hiera data merging",
			func(manifestData, hieraData string, check func(map[string]any)) {
				manifestContent := `
data:
` + manifestData + `
ccm:
  resources:
    - package:
        name: test-pkg
        ensure: present
`
				manifestPath := tempDir + "/hiera-merge-test.yaml"
				err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				hieraContent := `
data:
` + hieraData
				hieraPath := tempDir + "/hiera-override.yaml"
				err = os.WriteFile(hieraPath, []byte(hieraContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath,
					WithOverridingHieraData(hieraPath))
				Expect(err).NotTo(HaveOccurred())
				Expect(apply).NotTo(BeNil())

				check(apply.Data())
			},
			Entry("override existing value",
				`  app_name: original
  log_level: INFO`,
				`  app_name: overridden`,
				func(data map[string]any) {
					Expect(data).To(HaveKeyWithValue("app_name", "overridden"))
					Expect(data).To(HaveKeyWithValue("log_level", "INFO"))
				},
			),
			Entry("augment with new key",
				`  existing_key: original_value`,
				`  new_key: added_value`,
				func(data map[string]any) {
					Expect(data).To(HaveKeyWithValue("existing_key", "original_value"))
					Expect(data).To(HaveKeyWithValue("new_key", "added_value"))
				},
			),
			Entry("deep merge nested structures",
				`  config:
    database:
      host: localhost
      port: 5432
    cache:
      enabled: true`,
				`  config:
    database:
      host: production-db`,
				func(data map[string]any) {
					config, ok := data["config"].(map[string]any)
					Expect(ok).To(BeTrue())

					database, ok := config["database"].(map[string]any)
					Expect(ok).To(BeTrue())
					Expect(database["host"]).To(Equal("production-db"))
					Expect(database["port"]).To(BeNumerically("==", 5432))

					cache, ok := config["cache"].(map[string]any)
					Expect(ok).To(BeTrue())
					Expect(cache["enabled"]).To(BeTrue())
				},
			),
		)
	})

	Describe("WithOverridingResolvedData", func() {
		It("should set the overridingResolvedData field", func() {
			apply := &Apply{}
			overrideData := map[string]any{"key": "value"}
			opt := WithOverridingResolvedData(overrideData)
			err := opt(apply)
			Expect(err).NotTo(HaveOccurred())
			Expect(apply.overridingResolvedData).To(Equal(overrideData))
		})

		It("should merge overriding data into resolved data", func() {
			manifestContent := `
data:
  app_name: original
  log_level: INFO
ccm:
  resources:
    - package:
        name: test-pkg
        ensure: present
`
			manifestPath := tempDir + "/override-resolved-test.yaml"
			err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			overrideData := map[string]any{
				"app_name":  "overridden",
				"extra_key": "extra_value",
			}

			_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath,
				WithOverridingResolvedData(overrideData))
			Expect(err).NotTo(HaveOccurred())
			Expect(apply).NotTo(BeNil())

			data := apply.Data()
			Expect(data).To(HaveKeyWithValue("app_name", "overridden"))
			Expect(data).To(HaveKeyWithValue("log_level", "INFO"))
			Expect(data).To(HaveKeyWithValue("extra_key", "extra_value"))
		})

		It("should deep merge nested structures", func() {
			manifestContent := `
data:
  config:
    database:
      host: localhost
      port: 5432
    cache:
      enabled: true
ccm:
  resources:
    - package:
        name: test-pkg
        ensure: present
`
			manifestPath := tempDir + "/override-nested-test.yaml"
			err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			overrideData := map[string]any{
				"config": map[string]any{
					"database": map[string]any{
						"host": "production-db",
					},
				},
			}

			_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath,
				WithOverridingResolvedData(overrideData))
			Expect(err).NotTo(HaveOccurred())
			Expect(apply).NotTo(BeNil())

			data := apply.Data()
			config, ok := data["config"].(map[string]any)
			Expect(ok).To(BeTrue())

			database, ok := config["database"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(database["host"]).To(Equal("production-db"))
			Expect(database["port"]).To(BeNumerically("==", 5432))

			cache, ok := config["cache"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(cache["enabled"]).To(BeTrue())
		})

		It("should apply after hiera overrides", func() {
			manifestContent := `
data:
  key1: original1
  key2: original2
ccm:
  resources:
    - package:
        name: test-pkg
        ensure: present
`
			manifestPath := tempDir + "/combined-override-test.yaml"
			err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			hieraContent := `
data:
  key1: hiera_value
  key3: hiera_only
`
			hieraPath := tempDir + "/hiera-for-combined.yaml"
			err = os.WriteFile(hieraPath, []byte(hieraContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			resolvedOverride := map[string]any{
				"key1": "resolved_final",
				"key4": "resolved_only",
			}

			_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath,
				WithOverridingHieraData(hieraPath),
				WithOverridingResolvedData(resolvedOverride))
			Expect(err).NotTo(HaveOccurred())
			Expect(apply).NotTo(BeNil())

			data := apply.Data()
			// key1 should have the resolved override value (applied last)
			Expect(data).To(HaveKeyWithValue("key1", "resolved_final"))
			// key2 should have the original manifest value
			Expect(data).To(HaveKeyWithValue("key2", "original2"))
			// key3 should have the hiera override value
			Expect(data).To(HaveKeyWithValue("key3", "hiera_only"))
			// key4 should have the resolved override value
			Expect(data).To(HaveKeyWithValue("key4", "resolved_only"))
		})

		It("should handle nil overriding data gracefully", func() {
			manifestContent := `
data:
  key: value
ccm:
  resources:
    - package:
        name: test-pkg
        ensure: present
`
			manifestPath := tempDir + "/nil-override-test.yaml"
			err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// nil override should not cause issues
			_, apply, err := ResolveManifestFilePath(ctx, mockMgr, manifestPath,
				WithOverridingResolvedData(nil))
			Expect(err).NotTo(HaveOccurred())
			Expect(apply).NotTo(BeNil())

			data := apply.Data()
			Expect(data).To(HaveKeyWithValue("key", "value"))
		})
	})
})

var _ = Describe("validateManifest", func() {
	var apply *Apply

	BeforeEach(func() {
		apply = &Apply{}
	})

	Describe("validateManifest", func() {
		It("Should return error for empty manifest", func() {
			err := apply.validateManifest([]byte{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("manifest not parsed"))
		})

		It("Should validate a minimal valid manifest", func() {
			manifest := `
ccm:
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate manifest with all top-level sections", func() {
			manifest := `
data:
  package_name: httpd
  service_name: httpd
ccm:
  fail_on_error: true
  resources:
    - package:
        - httpd:
            ensure: present
hierarchy:
  order:
    - "os:linux"
  merge: deep
overrides:
  "os:linux":
    package_name: apache2
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate manifest without optional sections", func() {
			manifest := `
ccm:
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject unknown top-level properties", func() {
			manifest := `
unknown_property: value
ccm:
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should reject unknown ccm properties", func() {
			manifest := `
ccm:
  unknown_field: true
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should reject unknown resource types", func() {
			manifest := `
ccm:
  resources:
    - unknown_type:
        - something:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate package resource properties", func() {
			manifest := `
ccm:
  resources:
    - package:
        - vim:
            ensure: present
            provider: apt
            alias: editor
            require:
              - file#/etc/apt/sources.list
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject unknown package resource properties", func() {
			manifest := `
ccm:
  resources:
    - package:
        - vim:
            ensure: present
            unknown_prop: value
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate service resource properties", func() {
			manifest := `
ccm:
  resources:
    - service:
        - nginx:
            ensure: running
            enable: true
            subscribe:
              - file#/etc/nginx/nginx.conf
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject invalid service ensure value", func() {
			manifest := `
ccm:
  resources:
    - service:
        - nginx:
            ensure: invalid
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate file resource properties", func() {
			manifest := `
ccm:
  resources:
    - file:
        - /etc/config.conf:
            ensure: present
            content: "config data"
            owner: root
            group: root
            mode: "0644"
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject invalid file ensure value", func() {
			manifest := `
ccm:
  resources:
    - file:
        - /etc/config.conf:
            ensure: invalid
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate exec resource properties", func() {
			manifest := `
ccm:
  resources:
    - exec:
        - "/usr/bin/setup.sh":
            ensure: present
            cwd: /tmp
            timeout: 30s
            creates: /tmp/done
            refreshonly: true
            logoutput: true
            environment:
              - FOO=bar
            returns:
              - 0
              - 1
            subscribe:
              - file#/etc/config.conf
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject unknown exec resource properties", func() {
			manifest := `
ccm:
  resources:
    - exec:
        - "/usr/bin/setup.sh":
            ensure: present
            unknown_prop: value
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate hierarchy merge values", func() {
			manifest := `
hierarchy:
  order:
    - default
  merge: first
ccm:
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject invalid hierarchy merge value", func() {
			manifest := `
hierarchy:
  order:
    - default
  merge: invalid
ccm:
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate health_checks on resources", func() {
			manifest := `
ccm:
  resources:
    - package:
        - nginx:
            ensure: present
            health_checks:
              - command: "curl -s localhost"
                timeout: 10s
                tries: 3
                try_sleep: 1s
                format: nagios
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject health_check without command", func() {
			manifest := `
ccm:
  resources:
    - package:
        - nginx:
            ensure: present
            health_checks:
              - timeout: 10s
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})

		It("Should validate resources_jet_file property", func() {
			manifest := `
ccm:
  resources_jet_file: resources.jet
  resources:
    - package:
        - vim:
            ensure: present
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject invalid YAML", func() {
			manifest := `
ccm:
  resources: [unclosed
`
			err := apply.validateManifest([]byte(manifest))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("validateManifestAny", func() {
		It("Should validate a valid manifest struct", func() {
			manifest := map[string]any{
				"ccm": map[string]any{
					"fail_on_error": true,
					"resources": []any{
						map[string]any{
							"package": []any{
								map[string]any{
									"vim": map[string]any{
										"ensure": "present",
									},
								},
							},
						},
					},
				},
				"data": map[string]any{
					"key": "value",
				},
			}

			err := apply.validateManifestAny(manifest)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject invalid manifest struct", func() {
			manifest := map[string]any{
				"ccm": map[string]any{
					"unknown_field": true,
					"resources": []any{
						map[string]any{
							"package": []any{
								map[string]any{
									"vim": map[string]any{
										"ensure": "present",
									},
								},
							},
						},
					},
				},
			}

			err := apply.validateManifestAny(manifest)
			Expect(err).To(HaveOccurred())
		})

		It("Should validate complex manifest with all resource types", func() {
			manifest := map[string]any{
				"data": map[string]any{
					"app_name": "myapp",
				},
				"hierarchy": map[string]any{
					"order": []any{"default"},
					"merge": "deep",
				},
				"overrides": map[string]any{
					"default": map[string]any{
						"app_name": "overridden",
					},
				},
				"ccm": map[string]any{
					"fail_on_error": false,
					"resources": []any{
						map[string]any{
							"package": []any{
								map[string]any{
									"nginx": map[string]any{
										"ensure": "present",
									},
								},
							},
						},
						map[string]any{
							"file": []any{
								map[string]any{
									"/etc/nginx/nginx.conf": map[string]any{
										"ensure":  "present",
										"content": "server {}",
										"owner":   "root",
										"group":   "root",
										"mode":    "0644",
									},
								},
							},
						},
						map[string]any{
							"service": []any{
								map[string]any{
									"nginx": map[string]any{
										"ensure": "running",
										"enable": true,
										"subscribe": []any{
											"file#/etc/nginx/nginx.conf",
										},
									},
								},
							},
						},
						map[string]any{
							"exec": []any{
								map[string]any{
									"nginx -t": map[string]any{
										"ensure":      "present",
										"refreshonly": true,
										"subscribe": []any{
											"file#/etc/nginx/nginx.conf",
										},
									},
								},
							},
						},
					},
				},
			}

			err := apply.validateManifestAny(manifest)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
