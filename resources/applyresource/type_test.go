// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package applyresource

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestApplyResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/ApplyResource")
}

var _ = Describe("Apply Type", func() {
	var (
		facts    = make(map[string]any)
		data     = make(map[string]any)
		mgr      *modelmocks.MockManager
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		mockctl  *gomock.Controller
		provider *MockApplyProvider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr, logger = modelmocks.NewManager(facts, data, false, mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)
		mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
		provider = NewMockApplyProvider(mockctl)

		provider.EXPECT().Name().Return("mock").AnyTimes()
		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("New", func() {
		It("Should reject empty name", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ApplyResourceProperties{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name"))
		})

		It("Should reject missing ensure", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/etc/ccm/child.yaml",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ensure"))
		})

		It("Should reject ensure absent", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/etc/ccm/child.yaml",
					Ensure: model.EnsureAbsent,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid ensure value"))
		})

		It("Should reject URL names", func(ctx context.Context) {
			_, err := New(ctx, mgr, model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "https://example.com/manifest.yaml",
					Ensure: model.EnsurePresent,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("file path, not a URL"))
		})

		It("Should create resource with valid properties", func(ctx context.Context) {
			res, err := New(ctx, mgr, model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/etc/ccm/child.yaml",
					Ensure: model.EnsurePresent,
				},
				AllowApply: true,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.prop.Name).To(Equal("/etc/ccm/child.yaml"))
			Expect(res.prop.CommonResourceProperties.Type).To(Equal(model.ApplyTypeName))
		})

		It("Should create resource with relative path", func(ctx context.Context) {
			res, err := New(ctx, mgr, model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "manifests/child.yaml",
					Ensure: model.EnsurePresent,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.prop.Name).To(Equal("manifests/child.yaml"))
		})

		It("Should set alias from properties", func(ctx context.Context) {
			res, err := New(ctx, mgr, model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:   "/etc/ccm/child.yaml",
					Ensure: model.EnsurePresent,
					Alias:  "child",
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Base.CommonProperties.Alias).To(Equal("child"))

			event := res.NewTransactionEvent()
			Expect(event.Alias).To(Equal("child"))
		})
	})

	Context("with a prepared provider", func() {
		var factory *modelmocks.MockProviderFactory
		var applyRes *Type
		var err error

		BeforeEach(func(ctx context.Context) {
			factory = modelmocks.NewMockProviderFactory(mockctl)
			factory.EXPECT().Name().Return("test").AnyTimes()
			factory.EXPECT().TypeName().Return(model.ApplyTypeName).AnyTimes()
			factory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
				return provider, nil
			})

			properties := &model.ApplyResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name:     "/etc/ccm/child.yaml",
					Ensure:   model.EnsurePresent,
					Provider: "test",
				},
				AllowApply: true,
			}
			applyRes, err = New(ctx, mgr, *properties)
			Expect(err).ToNot(HaveOccurred())

			registry.Clear()
			registry.MustRegister(factory)
		})

		AfterEach(func() {
			registry.Clear()
		})

		Describe("Apply", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()
			})

			It("Should delegate to provider ApplyManifest", func(ctx context.Context) {
				finalState := &model.ApplyState{
					CommonResourceState: model.NewCommonResourceState(model.ResourceStatusApplyProtocol, model.ApplyTypeName, "/etc/ccm/child.yaml", model.EnsurePresent),
					ResourceCount:       3,
				}

				provider.EXPECT().ApplyManifest(gomock.Any(), mgr, gomock.Any(), 0, false, gomock.Any()).Return(finalState, nil)

				event, err := applyRes.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Changed).To(BeTrue())
				Expect(event.Failed).To(BeFalse())
			})

			It("Should report failure when ApplyManifest errors", func(ctx context.Context) {
				provider.EXPECT().ApplyManifest(gomock.Any(), mgr, gomock.Any(), 0, false, gomock.Any()).Return(nil, fmt.Errorf("child manifest failed"))

				event, err := applyRes.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Failed).To(BeTrue())
				Expect(event.Errors).To(ContainElement(ContainSubstring("child manifest failed")))
			})
		})

		Describe("Noop", func() {
			var noopMgr *modelmocks.MockManager
			var noopLogger *modelmocks.MockLogger
			var noopRunner *modelmocks.MockCommandRunner
			var noopProvider *MockApplyProvider
			var noopRes *Type

			BeforeEach(func(ctx context.Context) {
				noopMgr, noopLogger = modelmocks.NewManager(facts, data, true, mockctl)
				noopRunner = modelmocks.NewMockCommandRunner(mockctl)
				noopMgr.EXPECT().NewRunner().AnyTimes().Return(noopRunner, nil)
				noopProvider = NewMockApplyProvider(mockctl)
				noopProvider.EXPECT().Name().Return("mock").AnyTimes()
				noopLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
				noopLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

				noopFactory := modelmocks.NewMockProviderFactory(mockctl)
				noopFactory.EXPECT().Name().Return("noop-test").AnyTimes()
				noopFactory.EXPECT().TypeName().Return(model.ApplyTypeName).AnyTimes()
				noopFactory.EXPECT().New(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
					return noopProvider, nil
				})
				noopFactory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()

				registry.Clear()
				registry.MustRegister(noopFactory)

				noopProperties := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:     "/etc/ccm/child.yaml",
						Ensure:   model.EnsurePresent,
						Provider: "noop-test",
					},
					AllowApply: true,
				}

				var err error
				noopRes, err = New(ctx, noopMgr, *noopProperties)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should call ApplyManifest in noop mode to recurse into child resources", func(ctx context.Context) {
				finalState := &model.ApplyState{
					CommonResourceState: model.NewCommonResourceState(model.ResourceStatusApplyProtocol, model.ApplyTypeName, "/etc/ccm/child.yaml", model.EnsurePresent),
					ResourceCount:       2,
				}

				noopProvider.EXPECT().ApplyManifest(gomock.Any(), noopMgr, gomock.Any(), 0, false, gomock.Any()).Return(finalState, nil)

				event, err := noopRes.Apply(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Changed).To(BeTrue())
			})
		})

		Describe("Info", func() {
			It("Should return not supported error", func(ctx context.Context) {
				_, err := applyRes.Info(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("apply resources do not support info queries"))
			})
		})

		Describe("SelectProvider", func() {
			BeforeEach(func() {
				factory.EXPECT().IsManageable(facts, gomock.Any()).Return(true, 1, nil).AnyTimes()
			})

			It("Should select the registered provider", func() {
				name, err := applyRes.SelectProvider()
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal("mock"))
			})
		})
	})
})
