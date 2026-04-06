// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package ccmmanifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"github.com/choria-io/ccm/resources/apply"
	execresource "github.com/choria-io/ccm/resources/exec"
	execposix "github.com/choria-io/ccm/resources/exec/posix"
)

func TestCcmManifestProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/ApplyResource/CcmManifest")
}

var _ = Describe("Provider", func() {
	var (
		mockctl  *gomock.Controller
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		provider *Provider
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().With(gomock.Any()).AnyTimes().Return(logger)

		var err error
		provider, err = NewProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("NewProvider", func() {
		It("Should create a provider", func() {
			p, err := NewProvider(logger, runner)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.log).To(Equal(logger))
			Expect(p.runner).To(Equal(runner))
		})
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("ccmmanifest"))
		})
	})

	Describe("ApplyManifest", func() {
		var (
			mgr            *modelmocks.MockManager
			session        *modelmocks.MockSessionStore
			tempDir        string
			facts          map[string]any
			data           map[string]any
			setDataHistory []map[string]any
		)

		writeManifest := func(content string) string {
			path := filepath.Join(tempDir, "manifest.yaml")
			err := os.WriteFile(path, []byte(content), 0644)
			Expect(err).ToNot(HaveOccurred())
			return path
		}

		simpleManifest := `
ccm:
  resources:
    - exec:
        name: /bin/true
        ensure: present
`

		BeforeEach(func() {
			apply.ResourceFactory = func(ctx context.Context, mgr model.Manager, props model.ResourceProperties) (model.Resource, error) {
				switch rprop := props.(type) {
				case *model.ExecResourceProperties:
					return execresource.New(ctx, mgr, *rprop)
				default:
					return nil, fmt.Errorf("unsupported resource type %T", rprop)
				}
			}

			facts = map[string]any{"os": "linux"}
			data = map[string]any{"key": "value"}
			mgr, _ = modelmocks.NewManager(facts, data, false, mockctl)
			session = modelmocks.NewMockSessionStore(mockctl)

			setDataHistory = nil

			mgr.EXPECT().NewRunner().AnyTimes().Return(runner, nil)
			mgr.EXPECT().SetData(gomock.Any()).DoAndReturn(func(d map[string]any) map[string]any {
				setDataHistory = append(setDataHistory, d)
				return d
			}).AnyTimes()

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).AnyTimes().Return([]byte{}, []byte{}, 0, nil)

			session.EXPECT().RecordEvent(gomock.Any()).AnyTimes().Return(nil)
			mgr.EXPECT().StartSession(gomock.Any()).AnyTimes().Return(session, nil)
			mgr.EXPECT().RecordEvent(gomock.Any()).AnyTimes().Return(nil)
			mgr.EXPECT().PublishRegistration(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
			mgr.EXPECT().IsResourceFailed(gomock.Any(), gomock.Any()).AnyTimes().Return(false, nil)
			mgr.EXPECT().ShouldRefresh(gomock.Any(), gomock.Any()).AnyTimes().Return(false, nil)

			registry.Clear()
			execposix.Register()
			Register()

			var err error
			tempDir, err = os.MkdirTemp("", "ccmmanifest-test-*")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
			registry.Clear()
		})

		Describe("state save and restore", func() {
			It("Should restore noop mode after execution", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				mgr.SetNoopMode(true)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(mgr.NoopMode()).To(BeTrue())
			})

			It("Should restore working directory after execution", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				mgr.SetWorkingDirectory("/original/path")

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(mgr.WorkingDirectory()).To(Equal("/original/path"))
			})

			It("Should restore state even when resolve fails", func(ctx context.Context) {
				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   "/nonexistent/manifest.yaml",
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				mgr.SetNoopMode(true)
				mgr.SetWorkingDirectory("/original")

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).To(HaveOccurred())
				Expect(mgr.NoopMode()).To(BeTrue())
				Expect(mgr.WorkingDirectory()).To(Equal("/original"))
			})
		})

		Describe("noop behavior", func() {
			It("Should strengthen noop when child requests it", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				Expect(mgr.NoopMode()).To(BeFalse())

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					Noop:       true,
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())

				// noop should be restored to false after execution
				Expect(mgr.NoopMode()).To(BeFalse())
			})

			It("Should preserve parent noop when parent is already noop", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				mgr.SetNoopMode(true)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					Noop:       false,
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(mgr.NoopMode()).To(BeTrue())
			})

			It("Should not weaken parent noop when child sets noop=false", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				mgr.SetNoopMode(true)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					Noop:       false,
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(mgr.NoopMode()).To(BeTrue())
			})
		})

		Describe("data handling", func() {
			It("Should merge data via WithOverridingResolvedData when properties have data", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				childData := map[string]any{"child_key": "child_value"}

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					Data:       childData,
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())

				// Data is passed via WithOverridingResolvedData which merges
				// into hiera-resolved data before SetData is called by
				// ResolveManifestReader. The merged result should contain
				// our child data.
				Expect(setDataHistory).ToNot(BeEmpty())
				found := false
				for _, d := range setDataHistory {
					if v, ok := d["child_key"]; ok && v == "child_value" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "child data should appear in SetData calls after hiera merge")
			})

			It("Should not override data when properties have nil data", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("health_check_only", func() {
			It("Should pass healthCheckOnly through when parent sets it", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				Expect(mgr.NoopMode()).To(BeFalse())
				_, err := provider.ApplyManifest(ctx, mgr, props, 0, true, logger)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should strengthen healthCheckOnly when child sets it", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					HealthCheckOnly: true,
					AllowApply:      true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		// NOTE: allow_apply deny/allow tests for manifests containing apply
		// resources are tested in resources/apply/apply_test.go via
		// WithDenyApplyResources. Testing here requires the JSON schema
		// (step 6) to include the apply resource type first.

		Describe("allow_apply", func() {
			It("Should pass WithDenyApplyResources when AllowApply is false", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: false,
				}

				// Manifest has no apply resources so deny has no effect,
				// but verifies the option is wired through without error
				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should not pass WithDenyApplyResources when AllowApply is true", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("resolve errors", func() {
			It("Should return error when manifest file does not exist", func(ctx context.Context) {
				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   "/nonexistent/manifest.yaml",
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).To(HaveOccurred())
			})

			It("Should return error for invalid manifest content", func(ctx context.Context) {
				path := writeManifest("not: valid: yaml: [[[")

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				_, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("result state", func() {
			It("Should return correct ApplyState", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				state, err := provider.ApplyManifest(ctx, mgr, props, 0, false, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(state).ToNot(BeNil())
				Expect(state.Protocol).To(Equal(model.ResourceStatusApplyProtocol))
				Expect(state.ResourceType).To(Equal(model.ApplyTypeName))
				Expect(state.Name).To(Equal(path))
				Expect(state.Ensure).To(Equal(model.EnsurePresent))
				Expect(state.ResourceCount).To(Equal(1))
			})
		})

		Describe("recursion depth", func() {
			It("Should pass incremented depth to child", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				// currentDepth=8 passes depth 9 to child, within default max of 10
				state, err := provider.ApplyManifest(ctx, mgr, props, 8, false, logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(state).ToNot(BeNil())
			})

			It("Should fail when depth would exceed limit", func(ctx context.Context) {
				path := writeManifest(simpleManifest)

				props := &model.ApplyResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name:   path,
						Ensure: model.EnsurePresent,
					},
					AllowApply: true,
				}

				// currentDepth=9, child gets depth 10 which equals DefaultMaxRecursionDepth
				_, err := provider.ApplyManifest(ctx, mgr, props, 9, false, logger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("maximum apply depth"))
			})
		})
	})

})
