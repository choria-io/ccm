// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal/Registry")
}

var _ = Describe("Registry", func() {
	var (
		mockctl  *gomock.Controller
		factory1 *modelmocks.MockProviderFactory
		factory2 *modelmocks.MockProviderFactory
		factory3 *modelmocks.MockProviderFactory
		logger   *modelmocks.MockLogger
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		Clear()
		logger = modelmocks.NewMockLogger(mockctl)

		factory1 = modelmocks.NewMockProviderFactory(mockctl)
		factory1.EXPECT().TypeName().Return("package").AnyTimes()
		factory1.EXPECT().Name().Return("apt").AnyTimes()

		factory2 = modelmocks.NewMockProviderFactory(mockctl)
		factory2.EXPECT().TypeName().Return("package").AnyTimes()
		factory2.EXPECT().Name().Return("yum").AnyTimes()

		factory3 = modelmocks.NewMockProviderFactory(mockctl)
		factory3.EXPECT().TypeName().Return("service").AnyTimes()
		factory3.EXPECT().Name().Return("systemd").AnyTimes()

		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
		Clear()
	})

	Describe("registerProvider", func() {
		It("Should register a provider factory", func() {
			err := registerProvider(factory1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should register multiple providers of the same type", func() {
			err := registerProvider(factory1)
			Expect(err).ToNot(HaveOccurred())

			err = registerProvider(factory2)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should register providers of different types", func() {
			err := registerProvider(factory1)
			Expect(err).ToNot(HaveOccurred())

			err = registerProvider(factory3)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should fail when registering duplicate provider", func() {
			err := registerProvider(factory1)
			Expect(err).ToNot(HaveOccurred())

			err = registerProvider(factory1)
			Expect(err).To(Equal(model.ErrDuplicateProvider))
		})
	})

	Describe("mustRegisterProvider", func() {
		It("Should register a provider factory", func() {
			Expect(func() {
				mustRegisterProvider(factory1)
			}).ToNot(Panic())
		})

		It("Should panic when registration fails", func() {
			mustRegisterProvider(factory1)

			Expect(func() {
				mustRegisterProvider(factory1)
			}).To(Panic())
		})
	})

	Describe("Clear", func() {
		It("Should remove all registered providers", func() {
			registerProvider(factory1)
			registerProvider(factory2)
			registerProvider(factory3)

			Expect(Types()).To(HaveLen(2))

			Clear()

			Expect(Types()).To(BeEmpty())
		})
	})

	Describe("Types", func() {
		It("Should return empty list when no providers registered", func() {
			types := Types()
			Expect(types).To(BeEmpty())
		})

		It("Should return list of registered type names", func() {
			registerProvider(factory1)
			registerProvider(factory3)

			types := Types()
			Expect(types).To(HaveLen(2))
			Expect(types).To(ConsistOf("package", "service"))
		})

		It("Should not duplicate type names for multiple providers", func() {
			registerProvider(factory1)
			registerProvider(factory2)

			types := Types()
			Expect(types).To(HaveLen(1))
			Expect(types).To(ConsistOf("package"))
		})
	})

	Describe("selectProviders", func() {
		var facts map[string]any

		BeforeEach(func() {
			facts = map[string]any{
				"os": "ubuntu",
			}
		})

		It("Should return empty list when type not found", func() {
			providers, err := selectProviders("nonexistent", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(BeEmpty())
		})

		It("Should return empty list when no providers are manageable", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, nil)
			registerProvider(factory1)

			providers, err := selectProviders("package", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(BeEmpty())
		})

		It("Should return manageable providers", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
			factory2.EXPECT().IsManageable(facts, nil).Return(false, 0, nil)
			registerProvider(factory1)
			registerProvider(factory2)

			providers, err := selectProviders("package", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0]).To(Equal(factory1))
		})

		It("Should return multiple manageable providers", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
			factory2.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
			registerProvider(factory1)
			registerProvider(factory2)

			providers, err := selectProviders("package", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(2))
			Expect(providers).To(ConsistOf(factory1, factory2))
		})

		It("Should skip providers that error during IsManageable check", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, fmt.Errorf("check failed"))
			factory2.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
			registerProvider(factory1)
			registerProvider(factory2)

			providers, err := selectProviders("package", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0]).To(Equal(factory2))
		})

		It("Should return providers sorted by priority (lowest first)", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(true, 10, nil)
			factory2.EXPECT().IsManageable(facts, nil).Return(true, 5, nil)
			registerProvider(factory1)
			registerProvider(factory2)

			providers, err := selectProviders("package", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(2))
			Expect(providers[0]).To(Equal(factory2)) // priority 5
			Expect(providers[1]).To(Equal(factory1)) // priority 10
		})

		It("Should return all providers with same priority", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(true, 5, nil)
			factory2.EXPECT().IsManageable(facts, nil).Return(true, 5, nil)
			registerProvider(factory1)
			registerProvider(factory2)

			providers, err := selectProviders("package", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(2))
			Expect(providers).To(ConsistOf(factory1, factory2))
		})
	})

	Describe("selectProvider", func() {
		var facts map[string]any

		BeforeEach(func() {
			facts = map[string]any{
				"os": "ubuntu",
			}
		})

		It("Should return nil when type not found", func() {
			provider, err := selectProvider("nonexistent", "apt", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).To(BeNil())
		})

		It("Should return error when provider not found", func() {
			registerProvider(factory1)

			provider, err := selectProvider("package", "nonexistent", facts, nil, logger)
			Expect(err).To(Equal(model.ErrProviderNotFound))
			Expect(provider).To(BeNil())
		})

		It("Should return error when provider is not manageable", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, nil)
			registerProvider(factory1)

			provider, err := selectProvider("package", "apt", facts, nil, logger)
			Expect(err).To(MatchError(ContainSubstring("not applicable to instance")))
			Expect(provider).To(BeNil())
		})

		It("Should return error when IsManageable check fails", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, fmt.Errorf("check failed"))
			registerProvider(factory1)

			provider, err := selectProvider("package", "apt", facts, nil, logger)
			Expect(err).To(MatchError(ContainSubstring("check failed")))
			Expect(provider).To(BeNil())
		})

		It("Should return provider when found and manageable", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
			registerProvider(factory1)

			provider, err := selectProvider("package", "apt", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).To(Equal(factory1))
		})

		It("Should select correct provider from multiple registered", func() {
			factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
			registerProvider(factory1)
			registerProvider(factory2)

			provider, err := selectProvider("package", "apt", facts, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).To(Equal(factory1))
		})
	})

	Describe("FindSuitableProvider", func() {
		var (
			facts    map[string]any
			runner   *modelmocks.MockCommandRunner
			provider *modelmocks.MockProvider
		)

		BeforeEach(func() {
			facts = map[string]any{
				"os": "ubuntu",
			}
			runner = modelmocks.NewMockCommandRunner(mockctl)
			provider = modelmocks.NewMockProvider(mockctl)
		})

		Context("with auto-selection (empty provider name)", func() {
			It("Should return ErrNoSuitableProvider when no providers registered", func() {
				result, err := FindSuitableProvider("package", "", facts, nil, logger, runner)
				Expect(err).To(Equal(model.ErrNoSuitableProvider))
				Expect(result).To(BeNil())
			})

			It("Should return ErrNoSuitableProvider when no providers are manageable", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, nil)
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "", facts, nil, logger, runner)
				Expect(err).To(Equal(model.ErrNoSuitableProvider))
				Expect(result).To(BeNil())
			})

			It("Should create and return provider when exactly one is manageable", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
				factory1.EXPECT().New(logger, runner).Return(provider, nil)
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "", facts, nil, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})

			It("Should select the one manageable provider from multiple registered", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
				factory1.EXPECT().New(logger, runner).Return(provider, nil)
				factory2.EXPECT().IsManageable(facts, nil).Return(false, 0, nil)
				registerProvider(factory1)
				registerProvider(factory2)

				result, err := FindSuitableProvider("package", "", facts, nil, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})

			It("Should select the highest priority provider when multiple are manageable", func() {
				provider2 := modelmocks.NewMockProvider(mockctl)

				// factory1 has lower priority (10), factory2 has higher priority (5)
				factory1.EXPECT().IsManageable(facts, nil).Return(true, 10, nil)
				factory2.EXPECT().IsManageable(facts, nil).Return(true, 5, nil)
				factory2.EXPECT().New(logger, runner).Return(provider2, nil)
				registerProvider(factory1)
				registerProvider(factory2)

				result, err := FindSuitableProvider("package", "", facts, nil, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider2)) // factory2 selected due to lower priority value
			})

			It("Should return error when provider New() fails", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
				factory1.EXPECT().New(logger, runner).Return(nil, fmt.Errorf("failed to create provider"))
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "", facts, nil, logger, runner)
				Expect(err).To(MatchError(ContainSubstring("failed to create provider")))
				Expect(result).To(BeNil())
			})
		})

		Context("with explicit provider selection", func() {
			It("Should return ErrNoSuitableProvider when type not found", func() {
				result, err := FindSuitableProvider("nonexistent", "apt", facts, nil, logger, runner)
				Expect(err).To(Equal(model.ErrNoSuitableProvider))
				Expect(result).To(BeNil())
			})

			It("Should return ErrResourceInvalid when provider not found", func() {
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "nonexistent", facts, nil, logger, runner)
				Expect(err).To(MatchError(model.ErrResourceInvalid))
				Expect(err).To(MatchError(ContainSubstring(model.ErrProviderNotFound.Error())))
				Expect(result).To(BeNil())
			})

			It("Should return ErrResourceInvalid when provider is not manageable", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, nil)
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, nil, logger, runner)
				Expect(err).To(MatchError(model.ErrResourceInvalid))
				Expect(err).To(MatchError(ContainSubstring("not applicable")))
				Expect(result).To(BeNil())
			})

			It("Should return ErrResourceInvalid when IsManageable check fails", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(false, 0, fmt.Errorf("check failed"))
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, nil, logger, runner)
				Expect(err).To(MatchError(model.ErrResourceInvalid))
				Expect(err).To(MatchError(ContainSubstring("check failed")))
				Expect(result).To(BeNil())
			})

			It("Should create and return provider when found and manageable", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
				factory1.EXPECT().New(logger, runner).Return(provider, nil)
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, nil, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})

			It("Should return error when provider New() fails", func() {
				factory1.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
				factory1.EXPECT().New(logger, runner).Return(nil, fmt.Errorf("initialization failed"))
				registerProvider(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, nil, logger, runner)
				Expect(err).To(MatchError(ContainSubstring("initialization failed")))
				Expect(result).To(BeNil())
			})

			It("Should select correct provider from multiple registered", func() {
				factory2.EXPECT().IsManageable(facts, nil).Return(true, 1, nil)
				factory2.EXPECT().New(logger, runner).Return(provider, nil)
				registerProvider(factory1)
				registerProvider(factory2)

				result, err := FindSuitableProvider("package", "yum", facts, nil, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})
		})
	})

	Describe("Thread safety", func() {
		It("Should handle concurrent operations", func() {
			// Set up IsManageable expectations since selectProviders may call it
			// if factories are registered before it runs
			factory1.EXPECT().IsManageable(gomock.Any(), gomock.Any()).Return(true, 1, nil).AnyTimes()
			factory2.EXPECT().IsManageable(gomock.Any(), gomock.Any()).Return(true, 1, nil).AnyTimes()

			done := make(chan bool, 4) // Buffered to prevent blocking if goroutine fails

			// Concurrent registrations
			go func() {
				defer GinkgoRecover()
				registerProvider(factory1)
				done <- true
			}()

			go func() {
				defer GinkgoRecover()
				registerProvider(factory2)
				done <- true
			}()

			// Concurrent reads
			go func() {
				defer GinkgoRecover()
				Types()
				done <- true
			}()

			go func() {
				defer GinkgoRecover()
				selectProviders("package", map[string]any{}, nil, logger)
				done <- true
			}()

			// Wait for all goroutines
			for i := 0; i < 4; i++ {
				<-done
			}
		})
	})
})
