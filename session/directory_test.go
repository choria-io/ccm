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

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

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

	Describe("StartSession", func() {
		It("Should create the directory if it doesn't exist", func() {
			newDir := filepath.Join(tempDir, "newsession")
			newStore, err := NewDirectorySessionStore(newDir, logger, writer)
			Expect(err).ToNot(HaveOccurred())

			// Directory should not exist yet
			Expect(newDir).ToNot(BeADirectory())

			writer.EXPECT().Info("Creating new session record", gomock.Any(), gomock.Any()).Times(1)

			// StartSession should create it
			mockApply := modelmocks.NewMockApply(mockCtrl)
			mockApply.EXPECT().Resources().Return([]map[string]model.ResourceProperties{}).Times(1)
			err = newStore.StartSession(mockApply)
			Expect(err).ToNot(HaveOccurred())

			// Directory should now exist
			Expect(newDir).To(BeADirectory())
		})
	})

	Describe("RecordEvent", func() {
		It("Should write valid events to files", func() {
			event := model.NewTransactionEvent("package", "test")
			event.Changed = true

			err := store.RecordEvent(event)
			Expect(err).ToNot(HaveOccurred())

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

		It("Should fail when directory doesn't exist", func() {
			newDir := filepath.Join(tempDir, "nonexistent")
			newStore, err := NewDirectorySessionStore(newDir, logger, writer)
			Expect(err).ToNot(HaveOccurred())

			event := model.NewTransactionEvent("package", "test")
			err = newStore.RecordEvent(event)
			Expect(err).To(MatchError(ContainSubstring("does not exist")))
		})

		Context("Security - Directory Traversal Prevention", func() {
			It("Should reject invalid EventID with path traversal", func() {
				event := &model.TransactionEvent{
					EventID:      "../../../etc/passwd",
					Name:         "malicious",
					ResourceType: "attack",
				}

				err := store.RecordEvent(event)
				Expect(err).To(MatchError(ContainSubstring("invalid event ID")))

				// Verify no file was written
				files, err := os.ReadDir(tempDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(BeEmpty())
			})

			It("Should reject invalid EventID with absolute path", func() {
				event := &model.TransactionEvent{
					EventID:      "/tmp/malicious",
					Name:         "malicious",
					ResourceType: "attack",
				}

				err := store.RecordEvent(event)
				Expect(err).To(MatchError(ContainSubstring("invalid event ID")))

				// Verify it was not written to /tmp
				Expect("/tmp/malicious.event").ToNot(BeAnExistingFile())
			})

			It("Should reject invalid EventID with path separators", func() {
				event := &model.TransactionEvent{
					EventID:      "subdir/malicious",
					Name:         "malicious",
					ResourceType: "attack",
				}

				err := store.RecordEvent(event)
				Expect(err).To(MatchError(ContainSubstring("invalid event ID")))

				// Verify no subdirectory was created
				subdirPath := filepath.Join(tempDir, "subdir")
				Expect(subdirPath).ToNot(BeADirectory())
			})

			It("Should reject . as EventID", func() {
				event := &model.TransactionEvent{
					EventID:      ".",
					Name:         "malicious",
					ResourceType: "attack",
				}

				err := store.RecordEvent(event)
				Expect(err).To(MatchError(ContainSubstring("invalid event ID")))
			})

			It("Should reject .. as EventID", func() {
				event := &model.TransactionEvent{
					EventID:      "..",
					Name:         "malicious",
					ResourceType: "attack",
				}

				err := store.RecordEvent(event)
				Expect(err).To(MatchError(ContainSubstring("invalid event ID")))
			})

			It("Should reject empty EventID", func() {
				event := &model.TransactionEvent{
					EventID:      "",
					Name:         "malicious",
					ResourceType: "attack",
				}

				err := store.RecordEvent(event)
				Expect(err).To(MatchError(ContainSubstring("invalid event ID")))
			})
		})

		Context("Legitimate ksuid EventIDs", func() {
			It("Should accept valid ksuid EventIDs", func() {
				event := model.NewTransactionEvent("package", "test")

				err := store.RecordEvent(event)
				Expect(err).ToNot(HaveOccurred())

				expectedFile := filepath.Join(tempDir, event.EventID+".event")
				Expect(expectedFile).To(BeARegularFile())
			})
		})

		It("Should handle write errors gracefully", func() {
			// Create a writable directory first
			readOnlyDir := filepath.Join(tempDir, "readonly")
			err := os.Mkdir(readOnlyDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			// Now make it read-only
			err = os.Chmod(readOnlyDir, 0444)
			Expect(err).ToNot(HaveOccurred())

			badStore, err := NewDirectorySessionStore(readOnlyDir, logger, writer)
			Expect(err).ToNot(HaveOccurred())

			event := model.NewTransactionEvent("package", "test")
			err = badStore.RecordEvent(event)
			Expect(err).To(MatchError(ContainSubstring("permission denied")))

			// Cleanup
			os.Chmod(readOnlyDir, 0755)
		})
	})

	Describe("EventsForResource", func() {
		It("Should return events for a specific resource", func() {
			// Create multiple events
			event1 := model.NewTransactionEvent("package", "vim")
			event1.Changed = true
			Expect(store.RecordEvent(event1)).ToNot(HaveOccurred())

			event2 := model.NewTransactionEvent("package", "vim")
			event2.Changed = false
			Expect(store.RecordEvent(event2)).ToNot(HaveOccurred())

			event3 := model.NewTransactionEvent("package", "emacs")
			Expect(store.RecordEvent(event3)).ToNot(HaveOccurred())

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
			Expect(store.RecordEvent(event2)).ToNot(HaveOccurred())
			Expect(store.RecordEvent(event1)).ToNot(HaveOccurred())
			Expect(store.RecordEvent(event3)).ToNot(HaveOccurred())

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
			Expect(store.RecordEvent(pkg1)).ToNot(HaveOccurred())

			svc1 := model.NewTransactionEvent("service", "vim")
			Expect(store.RecordEvent(svc1)).ToNot(HaveOccurred())

			pkg2 := model.NewTransactionEvent("package", "emacs")
			Expect(store.RecordEvent(pkg2)).ToNot(HaveOccurred())

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
			Expect(store.RecordEvent(event1)).ToNot(HaveOccurred())

			// Write a corrupted file
			logger.EXPECT().Error("Failed to parse event type", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
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
