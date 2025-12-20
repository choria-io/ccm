// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry")
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
	})

	AfterEach(func() {
		mockctl.Finish()
		Clear()
	})

	Describe("Register", func() {
		It("Should register a provider factory", func() {
			err := Register(factory1)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should register multiple providers of the same type", func() {
			err := Register(factory1)
			Expect(err).ToNot(HaveOccurred())

			err = Register(factory2)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should register providers of different types", func() {
			err := Register(factory1)
			Expect(err).ToNot(HaveOccurred())

			err = Register(factory3)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should fail when registering duplicate provider", func() {
			err := Register(factory1)
			Expect(err).ToNot(HaveOccurred())

			err = Register(factory1)
			Expect(err).To(Equal(model.ErrDuplicateProvider))
		})
	})

	Describe("MustRegister", func() {
		It("Should register a provider factory", func() {
			Expect(func() {
				MustRegister(factory1)
			}).ToNot(Panic())
		})

		It("Should panic when registration fails", func() {
			MustRegister(factory1)

			Expect(func() {
				MustRegister(factory1)
			}).To(Panic())
		})
	})

	Describe("Clear", func() {
		It("Should remove all registered providers", func() {
			Register(factory1)
			Register(factory2)
			Register(factory3)

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
			Register(factory1)
			Register(factory3)

			types := Types()
			Expect(types).To(HaveLen(2))
			Expect(types).To(ConsistOf("package", "service"))
		})

		It("Should not duplicate type names for multiple providers", func() {
			Register(factory1)
			Register(factory2)

			types := Types()
			Expect(types).To(HaveLen(1))
			Expect(types).To(ConsistOf("package"))
		})
	})

	Describe("SelectProviders", func() {
		var facts map[string]any

		BeforeEach(func() {
			facts = map[string]any{
				"os": "ubuntu",
			}
		})

		It("Should return empty list when type not found", func() {
			providers, err := SelectProviders("nonexistent", facts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(BeEmpty())
		})

		It("Should return empty list when no providers are manageable", func() {
			factory1.EXPECT().IsManageable(facts).Return(false, nil)
			Register(factory1)

			providers, err := SelectProviders("package", facts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(BeEmpty())
		})

		It("Should return manageable providers", func() {
			factory1.EXPECT().IsManageable(facts).Return(true, nil)
			factory2.EXPECT().IsManageable(facts).Return(false, nil)
			Register(factory1)
			Register(factory2)

			providers, err := SelectProviders("package", facts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0]).To(Equal(factory1))
		})

		It("Should return multiple manageable providers", func() {
			factory1.EXPECT().IsManageable(facts).Return(true, nil)
			factory2.EXPECT().IsManageable(facts).Return(true, nil)
			Register(factory1)
			Register(factory2)

			providers, err := SelectProviders("package", facts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(2))
			Expect(providers).To(ConsistOf(factory1, factory2))
		})

		It("Should skip providers that error during IsManageable check", func() {
			factory1.EXPECT().IsManageable(facts).Return(false, fmt.Errorf("check failed"))
			factory2.EXPECT().IsManageable(facts).Return(true, nil)
			Register(factory1)
			Register(factory2)

			providers, err := SelectProviders("package", facts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0]).To(Equal(factory2))
		})
	})

	Describe("SelectProvider", func() {
		var facts map[string]any

		BeforeEach(func() {
			facts = map[string]any{
				"os": "ubuntu",
			}
		})

		It("Should return nil when type not found", func() {
			provider, err := SelectProvider("nonexistent", "apt", facts)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).To(BeNil())
		})

		It("Should return error when provider not found", func() {
			Register(factory1)

			provider, err := SelectProvider("package", "nonexistent", facts)
			Expect(err).To(Equal(model.ErrProviderNotFound))
			Expect(provider).To(BeNil())
		})

		It("Should return error when provider is not manageable", func() {
			factory1.EXPECT().IsManageable(facts).Return(false, nil)
			Register(factory1)

			provider, err := SelectProvider("package", "apt", facts)
			Expect(err).To(MatchError(ContainSubstring("not applicable to instance")))
			Expect(provider).To(BeNil())
		})

		It("Should return error when IsManageable check fails", func() {
			factory1.EXPECT().IsManageable(facts).Return(false, fmt.Errorf("check failed"))
			Register(factory1)

			provider, err := SelectProvider("package", "apt", facts)
			Expect(err).To(MatchError(ContainSubstring("check failed")))
			Expect(provider).To(BeNil())
		})

		It("Should return provider when found and manageable", func() {
			factory1.EXPECT().IsManageable(facts).Return(true, nil)
			Register(factory1)

			provider, err := SelectProvider("package", "apt", facts)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).To(Equal(factory1))
		})

		It("Should select correct provider from multiple registered", func() {
			factory1.EXPECT().IsManageable(facts).Return(true, nil)
			Register(factory1)
			Register(factory2)

			provider, err := SelectProvider("package", "apt", facts)
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
				result, err := FindSuitableProvider("package", "", facts, logger, runner)
				Expect(err).To(Equal(model.ErrNoSuitableProvider))
				Expect(result).To(BeNil())
			})

			It("Should return ErrNoSuitableProvider when no providers are manageable", func() {
				factory1.EXPECT().IsManageable(facts).Return(false, nil)
				Register(factory1)

				result, err := FindSuitableProvider("package", "", facts, logger, runner)
				Expect(err).To(Equal(model.ErrNoSuitableProvider))
				Expect(result).To(BeNil())
			})

			It("Should return ErrMultipleProviders when multiple providers are manageable", func() {
				factory1.EXPECT().IsManageable(facts).Return(true, nil)
				factory2.EXPECT().IsManageable(facts).Return(true, nil)
				Register(factory1)
				Register(factory2)

				result, err := FindSuitableProvider("package", "", facts, logger, runner)
				Expect(err).To(Equal(model.ErrMultipleProviders))
				Expect(result).To(BeNil())
			})

			It("Should create and return provider when exactly one is manageable", func() {
				factory1.EXPECT().IsManageable(facts).Return(true, nil)
				factory1.EXPECT().New(logger, runner).Return(provider, nil)
				Register(factory1)

				result, err := FindSuitableProvider("package", "", facts, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})

			It("Should select the one manageable provider from multiple registered", func() {
				factory1.EXPECT().IsManageable(facts).Return(true, nil)
				factory1.EXPECT().New(logger, runner).Return(provider, nil)
				factory2.EXPECT().IsManageable(facts).Return(false, nil)
				Register(factory1)
				Register(factory2)

				result, err := FindSuitableProvider("package", "", facts, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})

			It("Should return error when provider New() fails", func() {
				factory1.EXPECT().IsManageable(facts).Return(true, nil)
				factory1.EXPECT().New(logger, runner).Return(nil, fmt.Errorf("failed to create provider"))
				Register(factory1)

				result, err := FindSuitableProvider("package", "", facts, logger, runner)
				Expect(err).To(MatchError(ContainSubstring("failed to create provider")))
				Expect(result).To(BeNil())
			})
		})

		Context("with explicit provider selection", func() {
			It("Should return ErrNoSuitableProvider when type not found", func() {
				result, err := FindSuitableProvider("nonexistent", "apt", facts, logger, runner)
				Expect(err).To(Equal(model.ErrNoSuitableProvider))
				Expect(result).To(BeNil())
			})

			It("Should return ErrResourceInvalid when provider not found", func() {
				Register(factory1)

				result, err := FindSuitableProvider("package", "nonexistent", facts, logger, runner)
				Expect(err).To(MatchError(model.ErrResourceInvalid))
				Expect(err).To(MatchError(ContainSubstring(model.ErrProviderNotFound.Error())))
				Expect(result).To(BeNil())
			})

			It("Should return ErrResourceInvalid when provider is not manageable", func() {
				factory1.EXPECT().IsManageable(facts).Return(false, nil)
				Register(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, logger, runner)
				Expect(err).To(MatchError(model.ErrResourceInvalid))
				Expect(err).To(MatchError(ContainSubstring("not applicable")))
				Expect(result).To(BeNil())
			})

			It("Should return ErrResourceInvalid when IsManageable check fails", func() {
				factory1.EXPECT().IsManageable(facts).Return(false, fmt.Errorf("check failed"))
				Register(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, logger, runner)
				Expect(err).To(MatchError(model.ErrResourceInvalid))
				Expect(err).To(MatchError(ContainSubstring("check failed")))
				Expect(result).To(BeNil())
			})

			It("Should create and return provider when found and manageable", func() {
				factory1.EXPECT().IsManageable(facts).Return(true, nil)
				factory1.EXPECT().New(logger, runner).Return(provider, nil)
				Register(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})

			It("Should return error when provider New() fails", func() {
				factory1.EXPECT().IsManageable(facts).Return(true, nil)
				factory1.EXPECT().New(logger, runner).Return(nil, fmt.Errorf("initialization failed"))
				Register(factory1)

				result, err := FindSuitableProvider("package", "apt", facts, logger, runner)
				Expect(err).To(MatchError(ContainSubstring("initialization failed")))
				Expect(result).To(BeNil())
			})

			It("Should select correct provider from multiple registered", func() {
				factory2.EXPECT().IsManageable(facts).Return(true, nil)
				factory2.EXPECT().New(logger, runner).Return(provider, nil)
				Register(factory1)
				Register(factory2)

				result, err := FindSuitableProvider("package", "yum", facts, logger, runner)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(provider))
			})
		})
	})

	Describe("Thread safety", func() {
		It("Should handle concurrent operations", func() {
			// Set up IsManageable expectations since SelectProviders may call it
			// if factories are registered before it runs
			factory1.EXPECT().IsManageable(gomock.Any()).Return(true, nil).AnyTimes()
			factory2.EXPECT().IsManageable(gomock.Any()).Return(true, nil).AnyTimes()

			done := make(chan bool, 4) // Buffered to prevent blocking if goroutine fails

			// Concurrent registrations
			go func() {
				defer GinkgoRecover()
				Register(factory1)
				done <- true
			}()

			go func() {
				defer GinkgoRecover()
				Register(factory2)
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
				SelectProviders("package", map[string]any{}, logger)
				done <- true
			}()

			// Wait for all goroutines
			for i := 0; i < 4; i++ {
				<-done
			}
		})
	})
})
