// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestResources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources")
}

var _ = Describe("NewResourceFromProperties", func() {
	var (
		facts   = make(map[string]any)
		data    = make(map[string]any)
		mgr     *modelmocks.MockManager
		mockctl *gomock.Controller
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, _ = modelmocks.NewManager(facts, data, false, mockctl)
		runner := modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)

		registry.Clear()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("Package resource", func() {
		It("Should create a package resource from PackageResourceProperties", func(ctx context.Context) {
			props := &model.PackageResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "nginx",
					Ensure: model.EnsurePresent,
				},
			}

			resource, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource).ToNot(BeNil())
		})

		It("Should return validation error for invalid package properties", func(ctx context.Context) {
			props := &model.PackageResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "nginx",
					// Missing Ensure
				},
			}

			_, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})
	})

	Describe("Service resource", func() {
		It("Should create a service resource from ServiceResourceProperties", func(ctx context.Context) {
			props := &model.ServiceResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "nginx",
					Ensure: model.ServiceEnsureRunning,
				},
			}

			resource, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource).ToNot(BeNil())
		})

		It("Should return validation error for invalid service properties", func(ctx context.Context) {
			props := &model.ServiceResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					// Missing Name
					Ensure: model.ServiceEnsureRunning,
				},
			}

			_, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(model.ErrResourceNameRequired))
		})
	})

	Describe("File resource", func() {
		It("Should create a file resource from FileResourceProperties", func(ctx context.Context) {
			props := &model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/etc/test.conf",
					Ensure: model.EnsurePresent,
				},
				Owner: "root",
				Group: "root",
				Mode:  "0644",
			}

			resource, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource).ToNot(BeNil())
		})

		It("Should return validation error for invalid file properties", func(ctx context.Context) {
			props := &model.FileResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/etc/test.conf",
					// Missing Ensure
				},
				Owner: "root",
				Group: "root",
				Mode:  "0644",
			}

			_, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})
	})

	Describe("Exec resource", func() {
		It("Should create an exec resource from ExecResourceProperties", func(ctx context.Context) {
			props := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/bin/echo hello",
					Ensure: model.EnsurePresent,
				},
			}

			resource, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource).ToNot(BeNil())
		})

		It("Should return validation error for invalid exec properties", func(ctx context.Context) {
			props := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
					// Missing Ensure
				},
			}

			_, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(model.ErrResourceEnsureRequired))
		})
	})

	Describe("Archive resource", func() {
		It("Should create an archive resource from ArchiveResourceProperties", func(ctx context.Context) {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/myarchive.tar.gz",
					Ensure: model.EnsurePresent,
				},
				Url:           "https://example.com/archive.tar.gz",
				ExtractParent: "/opt",
				Owner:         "root",
				Group:         "root",
			}

			resource, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource).ToNot(BeNil())
		})

		It("Should return validation error for invalid archive properties", func(ctx context.Context) {
			props := &model.ArchiveResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/tmp/myarchive.tar.gz",
					Ensure: model.EnsurePresent,
				},
				// Missing required fields: Url, ExtractParent, Owner, Group
			}

			_, err := NewResourceFromProperties(ctx, mgr, props)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Unsupported resource type", func() {
		It("Should return error for nil properties", func(ctx context.Context) {
			_, err := NewResourceFromProperties(ctx, mgr, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported resource property type"))
		})
	})
})
