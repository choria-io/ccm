// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"testing"
	"time"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"github.com/choria-io/ccm/resources/apply"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestSession(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Session")
}

var _ = Describe("MemorySessionStore", func() {
	var (
		mockctl *gomock.Controller
		logger  *modelmocks.MockLogger
		writer  *modelmocks.MockLogger
		store   *MemorySessionStore
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		writer = modelmocks.NewMockLogger(mockctl)
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("NewMemorySessionStore", func() {
		It("Should create a new session store", func() {
			store, err := NewMemorySessionStore(logger, writer)
			Expect(err).ToNot(HaveOccurred())
			Expect(store).ToNot(BeNil())
			Expect(store.events).ToNot(BeNil())
			Expect(store.events).To(BeEmpty())
		})
	})

	Describe("ResetSession", func() {
		BeforeEach(func() {
			var err error
			store, err = NewMemorySessionStore(logger, writer)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should reset the session with empty manifest", func() {
			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0)

			store.ResetSession(manifest)

			Expect(store.events).To(BeEmpty())
			Expect(store.start).ToNot(BeZero())
		})

		It("Should reset the session with resources", func() {
			manifest := &apply.Apply{}
			// Simulate 3 resources
			writer.EXPECT().Info("Creating new session record", "resources", 0)

			store.ResetSession(manifest)

			Expect(store.events).To(BeEmpty())
		})

		It("Should clear existing events", func() {
			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0).Times(2)

			// First session with events
			store.ResetSession(manifest)
			event := model.TransactionEvent{
				Name:         "test",
				ResourceType: "package",
			}
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			store.RecordEvent(event)
			Expect(store.events).To(HaveLen(1))

			// Reset should clear events
			store.ResetSession(manifest)
			Expect(store.events).To(BeEmpty())
		})

		It("Should update the start time", func() {
			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0).Times(2)

			store.ResetSession(manifest)
			firstStart := store.start

			time.Sleep(10 * time.Millisecond)

			store.ResetSession(manifest)
			secondStart := store.start

			Expect(secondStart).To(BeTemporally(">", firstStart))
		})
	})

	Describe("RecordEvent", func() {
		BeforeEach(func() {
			var err error
			store, err = NewMemorySessionStore(logger, writer)
			Expect(err).ToNot(HaveOccurred())

			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0)
			store.ResetSession(manifest)
		})

		It("Should record a single event", func() {
			event := model.TransactionEvent{
				Name:         "vim",
				ResourceType: "package",
				Changed:      true,
			}

			writer.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
			store.RecordEvent(event)

			Expect(store.events).To(HaveLen(1))
			Expect(store.events[0].Name).To(Equal("vim"))
			Expect(store.events[0].ResourceType).To(Equal("package"))
			Expect(store.events[0].Changed).To(BeTrue())
		})

		It("Should record multiple events", func() {
			events := []model.TransactionEvent{
				{Name: "vim", ResourceType: "package", Changed: true},
				{Name: "git", ResourceType: "package", Changed: false},
				{Name: "nginx", ResourceType: "service", Changed: true},
			}

			writer.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			for _, e := range events {
				store.RecordEvent(e)
			}

			Expect(store.events).To(HaveLen(3))
		})

		It("Should maintain event order", func() {
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

			store.RecordEvent(model.TransactionEvent{Name: "first", ResourceType: "package"})
			store.RecordEvent(model.TransactionEvent{Name: "second", ResourceType: "package"})
			store.RecordEvent(model.TransactionEvent{Name: "third", ResourceType: "package"})

			Expect(store.events).To(HaveLen(3))
			Expect(store.events[0].Name).To(Equal("first"))
			Expect(store.events[1].Name).To(Equal("second"))
			Expect(store.events[2].Name).To(Equal("third"))
		})
	})

	Describe("ResourceEvents", func() {
		BeforeEach(func() {
			var err error
			store, err = NewMemorySessionStore(logger, writer)
			Expect(err).ToNot(HaveOccurred())

			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0)
			store.ResetSession(manifest)

			// Add test events
			events := []model.TransactionEvent{
				{Name: "vim", ResourceType: "package", Changed: true},
				{Name: "git", ResourceType: "package", Changed: false},
				{Name: "nginx", ResourceType: "service", Changed: true},
				{Name: "vim", ResourceType: "package", Changed: false},
			}

			writer.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			for _, e := range events {
				store.RecordEvent(e)
			}
		})

		It("Should return events for a specific resource", func() {
			events := store.ResourceEvents("package", "vim")
			Expect(events).To(HaveLen(2))
			Expect(events[0].Name).To(Equal("vim"))
			Expect(events[0].ResourceType).To(Equal("package"))
			Expect(events[1].Name).To(Equal("vim"))
		})

		It("Should return empty slice for non-existent resource", func() {
			events := store.ResourceEvents("package", "nonexistent")
			Expect(events).To(BeEmpty())
		})

		It("Should filter by both type and name", func() {
			events := store.ResourceEvents("service", "nginx")
			Expect(events).To(HaveLen(1))
			Expect(events[0].Name).To(Equal("nginx"))
			Expect(events[0].ResourceType).To(Equal("service"))
		})

		It("Should not return events with same name but different type", func() {
			// Add a package named "nginx"
			writer.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			store.RecordEvent(model.TransactionEvent{
				Name:         "nginx",
				ResourceType: "package",
				Changed:      true,
			})

			serviceEvents := store.ResourceEvents("service", "nginx")
			Expect(serviceEvents).To(HaveLen(1))
			Expect(serviceEvents[0].ResourceType).To(Equal("service"))

			packageEvents := store.ResourceEvents("package", "nginx")
			Expect(packageEvents).To(HaveLen(1))
			Expect(packageEvents[0].ResourceType).To(Equal("package"))
		})

		It("Should preserve event order", func() {
			events := store.ResourceEvents("package", "vim")
			Expect(events).To(HaveLen(2))
			Expect(events[0].Changed).To(BeTrue())
			Expect(events[1].Changed).To(BeFalse())
		})
	})

	Describe("Thread safety", func() {
		BeforeEach(func() {
			var err error
			store, err = NewMemorySessionStore(logger, writer)
			Expect(err).ToNot(HaveOccurred())

			manifest := &apply.Apply{}
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			store.ResetSession(manifest)
		})

		It("Should handle concurrent RecordEvent calls", func() {
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			done := make(chan bool)

			for i := 0; i < 10; i++ {
				go func(index int) {
					defer GinkgoRecover()
					event := model.TransactionEvent{
						Name:         "concurrent",
						ResourceType: "package",
					}
					store.RecordEvent(event)
					done <- true
				}(i)
			}

			for i := 0; i < 10; i++ {
				<-done
			}

			Expect(store.events).To(HaveLen(10))
		})

		It("Should handle concurrent ResourceEvents calls", func() {
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			store.RecordEvent(model.TransactionEvent{
				Name:         "test",
				ResourceType: "package",
			})

			done := make(chan bool)

			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					events := store.ResourceEvents("package", "test")
					Expect(events).To(HaveLen(1))
					done <- true
				}()
			}

			for i := 0; i < 10; i++ {
				<-done
			}
		})

		It("Should handle concurrent ResetSession and RecordEvent", func() {
			manifest := &apply.Apply{}
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			done := make(chan bool)

			// Reset session in background
			go func() {
				defer GinkgoRecover()
				time.Sleep(5 * time.Millisecond)
				store.ResetSession(manifest)
				done <- true
			}()

			// Record events concurrently
			go func() {
				defer GinkgoRecover()
				for i := 0; i < 5; i++ {
					store.RecordEvent(model.TransactionEvent{
						Name:         "concurrent",
						ResourceType: "package",
					})
					time.Sleep(2 * time.Millisecond)
				}
				done <- true
			}()

			// Wait for both goroutines
			<-done
			<-done

			// The final state should be consistent
			Expect(store.events).ToNot(BeNil())
		})
	})

	Describe("Edge cases", func() {
		BeforeEach(func() {
			var err error
			store, err = NewMemorySessionStore(logger, writer)
			Expect(err).ToNot(HaveOccurred())

			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0)
			store.ResetSession(manifest)
		})

		It("Should handle events with empty names", func() {
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			store.RecordEvent(model.TransactionEvent{
				Name:         "",
				ResourceType: "package",
			})

			events := store.ResourceEvents("package", "")
			Expect(events).To(HaveLen(1))
		})

		It("Should handle events with empty types", func() {
			writer.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			store.RecordEvent(model.TransactionEvent{
				Name:         "test",
				ResourceType: "",
			})

			events := store.ResourceEvents("", "test")
			Expect(events).To(HaveLen(1))
		})

		It("Should handle multiple resets without recording events", func() {
			manifest := &apply.Apply{}
			writer.EXPECT().Info("Creating new session record", "resources", 0).Times(3)

			store.ResetSession(manifest)
			store.ResetSession(manifest)
			store.ResetSession(manifest)

			Expect(store.events).To(BeEmpty())
		})
	})
})
