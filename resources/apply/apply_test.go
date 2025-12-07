// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"github.com/choria-io/ccm/templates"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
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

	Describe("ParseManifestHiera", func() {
		var (
			env *templates.Env
		)

		BeforeEach(func() {
			env = &templates.Env{
				Data:  map[string]any{"test": "value"},
				Facts: map[string]any{"os": "linux"},
			}
		})

		It("Should fail when resources key is missing", func() {
			resolved := map[string]any{}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no resources found"))
			Expect(apply).To(BeNil())
		})

		It("Should fail when resources is not an array", func() {
			resolved := map[string]any{
				"resources": "not-an-array",
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("resources must be an array"))
			Expect(apply).To(BeNil())
		})

		It("Should fail when resource is not a map", func() {
			resolved := map[string]any{
				"resources": []any{
					"not-a-map",
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("resources must be an array of maps"))
			Expect(apply).To(BeNil())
		})

		It("Should fail for unknown resource type", func() {
			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						"unknown": map[string]any{
							"name":   "test",
							"ensure": "present",
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown resource type"))
			Expect(apply).To(BeNil())
		})

		It("Should parse valid package resource", func() {
			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						model.PackageTypeName: map[string]any{
							"name":   "vim",
							"ensure": "present",
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(apply).ToNot(BeNil())
			Expect(apply.Resources()).To(HaveLen(1))
			Expect(apply.Data()).To(Equal(env.Data))
		})

		It("Should parse multiple package resources", func() {
			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						model.PackageTypeName: map[string]any{
							"name":   "vim",
							"ensure": "present",
						},
					},
					map[string]any{
						model.PackageTypeName: map[string]any{
							"name":   "git",
							"ensure": "latest",
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(apply).ToNot(BeNil())
			Expect(apply.Resources()).To(HaveLen(2))
		})

		It("Should fail when package properties are invalid", func() {
			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						model.PackageTypeName: map[string]any{
							"name": "vim",
							// Missing ensure
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ensure is required"))
			Expect(apply).To(BeNil())
		})

		It("Should fail when package name is missing", func() {
			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						model.PackageTypeName: map[string]any{
							"ensure": "present",
							// Missing name
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name is required"))
			Expect(apply).To(BeNil())
		})

		It("Should resolve templates in package properties", func() {
			env.Data["package_name"] = "nginx"
			env.Data["package_ensure"] = "latest"

			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						model.PackageTypeName: map[string]any{
							"name":   "{{ Data.package_name }}",
							"ensure": "{{ Data.package_ensure }}",
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(apply).ToNot(BeNil())
			Expect(apply.Resources()).To(HaveLen(1))

			pkg := apply.Resources()[0][model.PackageTypeName].(*model.PackageResourceProperties)
			Expect(pkg.Name).To(Equal("nginx"))
			Expect(pkg.Ensure).To(Equal("latest"))
		})

		It("Should fail when template syntax is invalid", func() {
			resolved := map[string]any{
				"resources": []any{
					map[string]any{
						model.PackageTypeName: map[string]any{
							"name":   "{{ invalid syntax }}}",
							"ensure": "present",
						},
					},
				},
			}

			apply, err := ParseManifestHiera(resolved, env, logger)
			Expect(err).To(HaveOccurred())
			Expect(apply).To(BeNil())
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
	})
})
