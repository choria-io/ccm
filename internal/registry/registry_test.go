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

	Describe("Thread safety", func() {
		It("Should handle concurrent operations", func() {
			done := make(chan bool)

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
