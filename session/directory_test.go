// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("DirectorySessionStore", func() {
	var (
		mockCtrl *gomock.Controller
		logger   *modelmocks.MockLogger
		writer   *modelmocks.MockLogger
		tempDir  string
		store    *DirectorySessionStore
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockCtrl)
		writer = modelmocks.NewMockLogger(mockCtrl)

		var err error
		tempDir = GinkgoT().TempDir()
		Expect(err).ToNot(HaveOccurred())

		store, err = NewDirectorySessionStore(tempDir, logger, writer)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("NewDirectorySessionStore", func() {
		It("Should create an absolute path from relative directory", func() {
			relStore, err := NewDirectorySessionStore("./relative/path", logger, writer)
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.IsAbs(relStore.directory)).To(BeTrue())
		})

		It("Should clean the directory path", func() {
			dirtyStore, err := NewDirectorySessionStore("/some//path/../clean/./path", logger, writer)
			Expect(err).ToNot(HaveOccurred())
			Expect(dirtyStore.directory).To(Equal("/some/clean/path"))
		})
	})

	Describe("RecordEvent", func() {
		It("Should write valid events to files", func() {
			event := model.NewTransactionEvent("package", "test")
			event.Changed = true

			store.RecordEvent(*event)

			expectedFile := filepath.Join(tempDir, event.EventID+".event")
			Expect(expectedFile).To(BeARegularFile())

			data, err := os.ReadFile(expectedFile)
			Expect(err).ToNot(HaveOccurred())

			var readEvent model.TransactionEvent
			err = json.Unmarshal(data, &readEvent)
			Expect(err).ToNot(HaveOccurred())
			Expect(readEvent.Name).To(Equal("test"))
			Expect(readEvent.Changed).To(BeTrue())
		})

		Context("Security - Directory Traversal Prevention", func() {
			It("Should reject invalid EventID with path traversal", func() {
				logger.EXPECT().Error("Invalid event ID, must be a valid ksuid", "event_id", "../../../etc/passwd").Times(1)

				event := model.TransactionEvent{
					EventID:      "../../../etc/passwd",
					Name:         "malicious",
					ResourceType: "attack",
				}

				store.RecordEvent(event)

				// Verify no file was written
				files, err := os.ReadDir(tempDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(BeEmpty())
			})

			It("Should reject invalid EventID with absolute path", func() {
				logger.EXPECT().Error("Invalid event ID, must be a valid ksuid", "event_id", "/tmp/malicious").Times(1)

				event := model.TransactionEvent{
					EventID:      "/tmp/malicious",
					Name:         "malicious",
					ResourceType: "attack",
				}

				store.RecordEvent(event)

				// Verify it was not written to /tmp
				Expect("/tmp/malicious.event").ToNot(BeAnExistingFile())
			})

			It("Should reject invalid EventID with path separators", func() {
				logger.EXPECT().Error("Invalid event ID, must be a valid ksuid", "event_id", "subdir/malicious").Times(1)

				event := model.TransactionEvent{
					EventID:      "subdir/malicious",
					Name:         "malicious",
					ResourceType: "attack",
				}

				store.RecordEvent(event)

				// Verify no subdirectory was created
				subdirPath := filepath.Join(tempDir, "subdir")
				Expect(subdirPath).ToNot(BeADirectory())
			})

			It("Should reject . as EventID", func() {
				logger.EXPECT().Error("Invalid event ID, must be a valid ksuid", "event_id", ".").Times(1)

				event := model.TransactionEvent{
					EventID:      ".",
					Name:         "malicious",
					ResourceType: "attack",
				}

				store.RecordEvent(event)
			})

			It("Should reject .. as EventID", func() {
				logger.EXPECT().Error("Invalid event ID, must be a valid ksuid", "event_id", "..").Times(1)

				event := model.TransactionEvent{
					EventID:      "..",
					Name:         "malicious",
					ResourceType: "attack",
				}

				store.RecordEvent(event)
			})

			It("Should reject empty EventID", func() {
				logger.EXPECT().Error("Invalid event ID, must be a valid ksuid", "event_id", "").Times(1)

				event := model.TransactionEvent{
					EventID:      "",
					Name:         "malicious",
					ResourceType: "attack",
				}

				store.RecordEvent(event)
			})
		})

		Context("Legitimate ksuid EventIDs", func() {
			It("Should accept valid ksuid EventIDs", func() {
				event := model.NewTransactionEvent("package", "test")

				store.RecordEvent(*event)

				expectedFile := filepath.Join(tempDir, event.EventID+".event")
				Expect(expectedFile).To(BeARegularFile())
			})
		})

		It("Should handle directory creation errors gracefully", func() {
			logger.EXPECT().Error("Failed to create session directory", "directory", gomock.Any(), "error", gomock.Any()).Times(1)

			// Create a read-only directory
			readOnlyDir := filepath.Join(tempDir, "readonly")
			err := os.Mkdir(readOnlyDir, 0444)
			Expect(err).ToNot(HaveOccurred())

			badStore, err := NewDirectorySessionStore(filepath.Join(readOnlyDir, "subdir"), logger, writer)
			Expect(err).ToNot(HaveOccurred())

			event := model.NewTransactionEvent("package", "test")
			badStore.RecordEvent(*event)

			// Cleanup
			os.Chmod(readOnlyDir, 0755)
		})
	})

	Describe("EventsForResource", func() {
		It("Should return events for a specific resource", func() {
			// Create multiple events
			event1 := model.NewTransactionEvent("package", "vim")
			event1.Changed = true
			store.RecordEvent(*event1)

			event2 := model.NewTransactionEvent("package", "vim")
			event2.Changed = false
			store.RecordEvent(*event2)

			event3 := model.NewTransactionEvent("package", "emacs")
			store.RecordEvent(*event3)

			// Get events for vim
			events, err := store.EventsForResource("package", "vim")
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(HaveLen(2))
			Expect(events[0].Name).To(Equal("vim"))
			Expect(events[1].Name).To(Equal("vim"))
		})

		It("Should return empty slice for non-existent resource", func() {
			events, err := store.EventsForResource("package", "nonexistent")
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(BeEmpty())
		})

		It("Should return empty slice when directory doesn't exist", func() {
			newStore, err := NewDirectorySessionStore(filepath.Join(tempDir, "nonexistent"), logger, writer)
			Expect(err).ToNot(HaveOccurred())

			events, err := newStore.EventsForResource("package", "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(BeEmpty())
		})

		It("Should sort events by time order (EventID)", func() {
			// Create events with different EventIDs
			event1 := model.NewTransactionEvent("package", "test")
			event2 := model.NewTransactionEvent("package", "test")
			event3 := model.NewTransactionEvent("package", "test")

			// Record in random order
			store.RecordEvent(*event2)
			store.RecordEvent(*event1)
			store.RecordEvent(*event3)

			events, err := store.EventsForResource("package", "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(HaveLen(3))

			// Verify they're sorted by EventID (time order)
			// ksuids are k-sortable strings, so lexicographic comparison gives time order
			Expect(events[0].EventID < events[1].EventID).To(BeTrue())
			Expect(events[1].EventID < events[2].EventID).To(BeTrue())
		})

		It("Should filter by both resourceType and name", func() {
			// Create events with different types and names
			pkg1 := model.NewTransactionEvent("package", "vim")
			store.RecordEvent(*pkg1)

			svc1 := model.NewTransactionEvent("service", "vim")
			store.RecordEvent(*svc1)

			pkg2 := model.NewTransactionEvent("package", "emacs")
			store.RecordEvent(*pkg2)

			// Get package vim events
			events, err := store.EventsForResource("package", "vim")
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(HaveLen(1))
			Expect(events[0].ResourceType).To(Equal("package"))
			Expect(events[0].Name).To(Equal("vim"))
		})

		It("Should skip corrupted event files", func() {
			// Write a valid event
			event1 := model.NewTransactionEvent("package", "test")
			store.RecordEvent(*event1)

			// Write a corrupted file
			logger.EXPECT().Error("Failed to parse event file", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			corruptedFile := filepath.Join(tempDir, "corrupted.event")
			err := os.WriteFile(corruptedFile, []byte("invalid json"), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Should still return the valid event
			events, err := store.EventsForResource("package", "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(events).To(HaveLen(1))
		})
	})
})
