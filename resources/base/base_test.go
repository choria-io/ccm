// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestBase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Base")
}

var _ = Describe("Base", func() {
	var (
		facts   = make(map[string]any)
		data    = make(map[string]any)
		mgr     *modelmocks.MockManager
		logger  *modelmocks.MockLogger
		runner  *modelmocks.MockCommandRunner
		mockctl *gomock.Controller
		mockRes *MockEmbeddedResource
		props   *model.FileResourceProperties
		b       *Base
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		mockRes = NewMockEmbeddedResource(mockctl)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

		props = &model.FileResourceProperties{
			CommonResourceProperties: model.CommonResourceProperties{
				Name:   "/tmp/testfile",
				Ensure: model.EnsurePresent,
			},
			Owner:    "root",
			Group:    "root",
			Mode:     "0644",
			Contents: "file content",
		}

		b = &Base{
			Resource:           mockRes,
			TypeName:           model.FileTypeName,
			InstanceName:       "/tmp/testfile",
			Ensure:             model.EnsurePresent,
			ResourceProperties: props,
			Log:                logger,
			Manager:            mgr,
		}
	})

	Describe("NewTransactionEvent", func() {
		It("Should create event with type and instance name", func() {
			event := b.NewTransactionEvent()
			Expect(event).ToNot(BeNil())
			Expect(event.ResourceType).To(Equal(model.FileTypeName))
			Expect(event.Name).To(Equal("/tmp/testfile"))
		})

		It("Should set properties from Base", func() {
			event := b.NewTransactionEvent()
			Expect(event.Properties).To(Equal(props))
			Expect(event.Ensure).To(Equal(model.EnsurePresent))
		})

		It("Should handle nil properties", func() {
			b.ResourceProperties = nil
			event := b.NewTransactionEvent()
			Expect(event).ToNot(BeNil())
			Expect(event.ResourceType).To(Equal(model.FileTypeName))
			Expect(event.Properties).To(BeNil())
		})

		It("Should set alias when InstanceAlias is set", func() {
			b.InstanceAlias = "motd"
			event := b.NewTransactionEvent()
			Expect(event).ToNot(BeNil())
			Expect(event.Alias).To(Equal("motd"))
			Expect(event.Name).To(Equal("/tmp/testfile"))
		})

		It("Should have empty alias when InstanceAlias is not set", func() {
			b.InstanceAlias = ""
			event := b.NewTransactionEvent()
			Expect(event).ToNot(BeNil())
			Expect(event.Alias).To(BeEmpty())
		})
	})

	Describe("Accessor methods", func() {
		It("Should return correct Type", func() {
			Expect(b.Type()).To(Equal(model.FileTypeName))
		})

		It("Should return correct Name", func() {
			Expect(b.Name()).To(Equal("/tmp/testfile"))
		})

		It("Should return correct String representation", func() {
			Expect(b.String()).To(Equal("file#/tmp/testfile"))
		})

		It("Should return properties", func() {
			Expect(b.Properties()).To(Equal(props))
		})
	})

	Describe("Healthcheck", func() {
		BeforeEach(func() {
			mockRes.EXPECT().SelectProvider().Return("mock", nil).AnyTimes()
			mockRes.EXPECT().NewTransactionEvent().DoAndReturn(func() *model.TransactionEvent {
				return model.NewTransactionEvent(model.FileTypeName, "/tmp/testfile", "")
			}).AnyTimes()
		})

		It("Should return non-failed event when no health check is configured", func(ctx context.Context) {
			props.HealthChecks = nil

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeFalse())
			Expect(result.HealthChecks).To(BeEmpty())
			Expect(result.Changed).To(BeFalse())
		})

		It("Should succeed when health check passes", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/test -f /tmp/testfile",
			}}

			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/test", "-f", "/tmp/testfile").
				Return([]byte("OK"), []byte{}, 0, nil)

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeFalse())
			Expect(result.HealthChecks).To(HaveLen(1))
			Expect(result.HealthChecks[0].Status).To(Equal(model.HealthCheckOK))
			Expect(result.Changed).To(BeFalse())
		})

		It("Should fail when health check fails", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/test -f /tmp/testfile",
			}}

			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/test", "-f", "/tmp/testfile").
				Return([]byte("CRITICAL"), []byte{}, 2, nil)

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeTrue())
			Expect(result.Errors).To(ContainElement(ContainSubstring("health check status")))
			Expect(result.HealthChecks).To(HaveLen(1))
			Expect(result.HealthChecks[0].Status).To(Equal(model.HealthCheckCritical))
			Expect(result.Changed).To(BeFalse())
		})

		It("Should fail when health check returns warning", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/check_something",
			}}

			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/check_something").
				Return([]byte("WARNING: something is not quite right"), []byte{}, 1, nil)

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeTrue())
			Expect(result.Errors).To(ContainElement(ContainSubstring("health check status")))
			Expect(result.HealthChecks).To(HaveLen(1))
			Expect(result.HealthChecks[0].Status).To(Equal(model.HealthCheckWarning))
		})

		It("Should fail when health check command execution fails", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/nonexistent",
			}}

			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/nonexistent").
				Return(nil, nil, 0, fmt.Errorf("command not found"))

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeTrue())
			Expect(result.Errors).To(ContainElement(ContainSubstring("command not found")))
		})

		It("Should not call ApplyResource", func(ctx context.Context) {
			// This test verifies that Healthcheck does NOT call apply logic
			// The mockRes has no ApplyResource expectation set, so any call would fail
			props.HealthChecks = nil

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeFalse())
		})

		It("Should capture health check output", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/check_disk",
			}}

			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/check_disk").
				Return([]byte("DISK OK - free space: 50%"), []byte{}, 0, nil)

			result, err := b.Healthcheck(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeFalse())
			Expect(result.HealthChecks[0].Output).To(Equal("DISK OK - free space: 50%"))
		})

		It("Should return error when SelectProvider fails", func(ctx context.Context) {
			// Create a new mock that will fail on SelectProvider
			failMock := NewMockEmbeddedResource(mockctl)
			failMock.EXPECT().SelectProvider().Return("", fmt.Errorf("no suitable provider"))

			failBase := &Base{
				Resource:           failMock,
				TypeName:           model.FileTypeName,
				InstanceName:       "/tmp/testfile",
				ResourceProperties: props,
				Log:                logger,
				Manager:            mgr,
			}

			_, err := failBase.Healthcheck(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no suitable provider"))
		})
	})

	Describe("Apply", func() {
		BeforeEach(func() {
			mockRes.EXPECT().SelectProvider().Return("mock", nil).AnyTimes()
			mockRes.EXPECT().NewTransactionEvent().DoAndReturn(func() *model.TransactionEvent {
				return model.NewTransactionEvent(model.FileTypeName, "/tmp/testfile", "")
			}).AnyTimes()
		})

		It("Should call ApplyResource and return state", func(ctx context.Context) {
			props.HealthChecks = nil
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{
					Ensure:  model.EnsurePresent,
					Changed: true,
				},
				Metadata: &model.FileMetadata{},
			}

			mockRes.EXPECT().ApplyResource(gomock.Any()).Return(state, nil)

			result, err := b.Apply(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Changed).To(BeTrue())
			Expect(result.ActualEnsure).To(Equal(model.EnsurePresent))
		})

		It("Should mark event as failed when ApplyResource fails", func(ctx context.Context) {
			props.HealthChecks = nil

			mockRes.EXPECT().ApplyResource(gomock.Any()).Return(nil, fmt.Errorf("apply failed"))

			result, err := b.Apply(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeTrue())
			Expect(result.Errors).To(ContainElement(ContainSubstring("apply failed")))
		})

		It("Should run health check after apply", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/test -f /tmp/testfile",
			}}
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{
					Ensure:  model.EnsurePresent,
					Changed: true,
				},
				Metadata: &model.FileMetadata{},
			}

			mockRes.EXPECT().ApplyResource(gomock.Any()).Return(state, nil)
			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/test", "-f", "/tmp/testfile").
				Return([]byte("OK"), []byte{}, 0, nil)

			result, err := b.Apply(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeFalse())
			Expect(result.Changed).To(BeTrue())
			Expect(result.HealthChecks).To(HaveLen(1))
			Expect(result.HealthChecks[0].Status).To(Equal(model.HealthCheckOK))
		})

		It("Should mark event as failed when health check fails after successful apply", func(ctx context.Context) {
			props.HealthChecks = []model.CommonHealthCheck{{
				Command: "/usr/bin/test -f /tmp/testfile",
			}}
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{
					Ensure:  model.EnsurePresent,
					Changed: true,
				},
				Metadata: &model.FileMetadata{},
			}

			mockRes.EXPECT().ApplyResource(gomock.Any()).Return(state, nil)
			runner.EXPECT().Execute(gomock.Any(), "/usr/bin/test", "-f", "/tmp/testfile").
				Return([]byte("CRITICAL"), []byte{}, 2, nil)

			result, err := b.Apply(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Failed).To(BeTrue())
			Expect(result.Errors).To(ContainElement(ContainSubstring("health check status")))
			Expect(result.Changed).To(BeTrue())
		})

		It("Should set noop fields from state", func(ctx context.Context) {
			props.HealthChecks = nil
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{
					Ensure:      model.EnsurePresent,
					Changed:     true,
					Noop:        true,
					NoopMessage: "Would have created the file",
				},
				Metadata: &model.FileMetadata{},
			}

			mockRes.EXPECT().ApplyResource(gomock.Any()).Return(state, nil)

			result, err := b.Apply(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Noop).To(BeTrue())
			Expect(result.NoopMessage).To(Equal("Would have created the file"))
		})

		It("Should return error when SelectProvider fails", func(ctx context.Context) {
			failMock := NewMockEmbeddedResource(mockctl)
			failMock.EXPECT().SelectProvider().Return("", fmt.Errorf("no suitable provider"))

			failBase := &Base{
				Resource:           failMock,
				TypeName:           model.FileTypeName,
				InstanceName:       "/tmp/testfile",
				ResourceProperties: props,
				Log:                logger,
				Manager:            mgr,
			}

			_, err := failBase.Apply(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no suitable provider"))
		})
	})

	Describe("FinalizeState", func() {
		It("Should set all state fields correctly", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{
					Ensure: model.EnsurePresent,
				},
				Metadata: &model.FileMetadata{},
			}

			b.FinalizeState(state, true, "Would have created", true, false, true)

			Expect(state.Noop).To(BeTrue())
			Expect(state.NoopMessage).To(Equal("Would have created"))
			Expect(state.Changed).To(BeTrue())
			Expect(state.Stable).To(BeFalse())
			Expect(state.Refreshed).To(BeTrue())
		})

		It("Should handle non-noop state", func() {
			state := &model.FileState{
				CommonResourceState: model.CommonResourceState{
					Ensure: model.EnsurePresent,
				},
				Metadata: &model.FileMetadata{},
			}

			b.FinalizeState(state, false, "", false, true, false)

			Expect(state.Noop).To(BeFalse())
			Expect(state.NoopMessage).To(BeEmpty())
			Expect(state.Changed).To(BeFalse())
			Expect(state.Stable).To(BeTrue())
			Expect(state.Refreshed).To(BeFalse())
		})
	})

	Describe("ShouldRefresh", func() {
		It("Should return false for empty subscribe list", func() {
			should, resource, err := b.ShouldRefresh([]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(should).To(BeFalse())
			Expect(resource).To(BeEmpty())
		})

		It("Should return true when a subscribed resource has changed", func() {
			mgr.EXPECT().ShouldRefresh("package", "nginx").Return(true, nil)

			should, resource, err := b.ShouldRefresh([]string{"package#nginx"})
			Expect(err).ToNot(HaveOccurred())
			Expect(should).To(BeTrue())
			Expect(resource).To(Equal("package#nginx"))
		})

		It("Should return false when no subscribed resource has changed", func() {
			mgr.EXPECT().ShouldRefresh("package", "nginx").Return(false, nil)
			mgr.EXPECT().ShouldRefresh("file", "/etc/nginx.conf").Return(false, nil)

			should, resource, err := b.ShouldRefresh([]string{"package#nginx", "file#/etc/nginx.conf"})
			Expect(err).ToNot(HaveOccurred())
			Expect(should).To(BeFalse())
			Expect(resource).To(BeEmpty())
		})

		It("Should return first changed resource when multiple have changed", func() {
			mgr.EXPECT().ShouldRefresh("package", "nginx").Return(false, nil)
			mgr.EXPECT().ShouldRefresh("file", "/etc/nginx.conf").Return(true, nil)

			should, resource, err := b.ShouldRefresh([]string{"package#nginx", "file#/etc/nginx.conf"})
			Expect(err).ToNot(HaveOccurred())
			Expect(should).To(BeTrue())
			Expect(resource).To(Equal("file#/etc/nginx.conf"))
		})

		It("Should return error from manager", func() {
			mgr.EXPECT().ShouldRefresh("package", "nginx").Return(false, fmt.Errorf("session error"))

			should, resource, err := b.ShouldRefresh([]string{"package#nginx"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("session error"))
			Expect(should).To(BeFalse())
			Expect(resource).To(Equal("package#nginx"))
		})
	})
})
