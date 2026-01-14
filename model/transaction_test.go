// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewTransactionEvent", func() {
	It("Should create event with all fields set", func() {
		event := NewTransactionEvent("package", "nginx", "webserver")
		Expect(event).ToNot(BeNil())
		Expect(event.ResourceType).To(Equal("package"))
		Expect(event.Name).To(Equal("nginx"))
		Expect(event.Alias).To(Equal("webserver"))
		Expect(event.Protocol).To(Equal(TransactionEventProtocol))
		Expect(event.EventID).ToNot(BeEmpty())
		Expect(event.TimeStamp).ToNot(BeZero())
	})

	It("Should handle empty alias", func() {
		event := NewTransactionEvent("service", "httpd", "")
		Expect(event.Alias).To(BeEmpty())
		Expect(event.Name).To(Equal("httpd"))
	})
})

var _ = Describe("TransactionEvent", func() {
	Describe("String", func() {
		It("Should show alias when set", func() {
			event := NewTransactionEvent("package", "nginx", "webserver")
			event.Changed = true
			event.RequestedEnsure = "present"
			event.Provider = "dnf"

			str := event.String()
			Expect(str).To(ContainSubstring("webserver (alias)"))
			Expect(str).ToNot(ContainSubstring("nginx"))
		})

		It("Should show name when alias is empty", func() {
			event := NewTransactionEvent("package", "nginx", "")
			event.Changed = true
			event.RequestedEnsure = "present"
			event.Provider = "dnf"

			str := event.String()
			Expect(str).To(ContainSubstring("package#nginx"))
			Expect(str).ToNot(ContainSubstring("(alias)"))
		})

		It("Should format failed event correctly", func() {
			event := NewTransactionEvent("service", "apache", "web")
			event.Failed = true
			event.RequestedEnsure = "running"
			event.Errors = []string{"service not found"}
			event.Provider = "systemd"

			str := event.String()
			Expect(str).To(ContainSubstring("service#web (alias)"))
			Expect(str).To(ContainSubstring("failed"))
			Expect(str).To(ContainSubstring("service not found"))
		})

		It("Should format skipped event correctly", func() {
			event := NewTransactionEvent("package", "vim", "editor")
			event.Skipped = true
			event.RequestedEnsure = "present"
			event.Provider = "dnf"

			str := event.String()
			Expect(str).To(ContainSubstring("package#editor (alias)"))
			Expect(str).To(ContainSubstring("skipped"))
		})

		It("Should format refreshed event correctly", func() {
			event := NewTransactionEvent("service", "nginx", "proxy")
			event.Refreshed = true
			event.RequestedEnsure = "running"
			event.Provider = "systemd"

			str := event.String()
			Expect(str).To(ContainSubstring("service#proxy (alias)"))
			Expect(str).To(ContainSubstring("refreshed"))
		})

		It("Should format stable event correctly", func() {
			event := NewTransactionEvent("file", "/etc/motd", "motd")
			event.RequestedEnsure = "present"
			event.Provider = "posix"

			str := event.String()
			Expect(str).To(ContainSubstring("file#motd (alias)"))
			Expect(str).To(ContainSubstring("ensure=present"))
			Expect(str).ToNot(ContainSubstring("changed"))
			Expect(str).ToNot(ContainSubstring("failed"))
		})
	})

})

var _ = Describe("SessionSummary", func() {
	Describe("BuildSessionSummary", func() {
		It("Should build a correct summary from events", func() {
			// Create a session start event
			startEvent := NewSessionStartEvent()
			startEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

			// Create various transaction events
			events := []SessionEvent{
				startEvent,
			}

			// Changed resource
			changedEvent := NewTransactionEvent("package", "nginx", "")
			changedEvent.Changed = true
			changedEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 5, 0, time.UTC)
			changedEvent.Duration = 5 * time.Second
			events = append(events, changedEvent)

			// Failed resource
			failedEvent := NewTransactionEvent("service", "apache", "")
			failedEvent.Failed = true
			failedEvent.Errors = append(failedEvent.Errors, "service not found")
			failedEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 10, 0, time.UTC)
			failedEvent.Duration = 2 * time.Second
			events = append(events, failedEvent)

			// Skipped resource
			skippedEvent := NewTransactionEvent("package", "vim", "")
			skippedEvent.Skipped = true
			skippedEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 12, 0, time.UTC)
			skippedEvent.Duration = 1 * time.Second
			events = append(events, skippedEvent)

			// Stable resource
			stableEvent := NewTransactionEvent("service", "sshd", "")
			stableEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 15, 0, time.UTC)
			stableEvent.Duration = 3 * time.Second
			events = append(events, stableEvent)

			// Refreshed resource
			refreshedEvent := NewTransactionEvent("service", "nginx", "")
			refreshedEvent.Changed = true
			refreshedEvent.Refreshed = true
			refreshedEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 20, 0, time.UTC)
			refreshedEvent.Duration = 2 * time.Second
			events = append(events, refreshedEvent)

			// Build summary
			summary := BuildSessionSummary(events)

			// Verify summary
			Expect(summary.TotalResources).To(Equal(5))
			Expect(summary.ChangedResources).To(Equal(2)) // nginx package and nginx service
			Expect(summary.FailedResources).To(Equal(1))  // apache service
			Expect(summary.SkippedResources).To(Equal(1)) // vim package
			Expect(summary.StableResources).To(Equal(1))  // sshd service
			Expect(summary.RefreshedCount).To(Equal(1))   // nginx service refreshed
			Expect(summary.TotalErrors).To(Equal(1))      // apache service error
			Expect(summary.StartTime).To(Equal(startEvent.TimeStamp))
			Expect(summary.EndTime).To(Equal(refreshedEvent.TimeStamp))
			Expect(summary.TotalDuration).To(Equal(20 * time.Second))
		})

		It("Should handle empty events", func() {
			summary := BuildSessionSummary([]SessionEvent{})

			Expect(summary.TotalResources).To(Equal(0))
			Expect(summary.ChangedResources).To(Equal(0))
			Expect(summary.FailedResources).To(Equal(0))
			Expect(summary.SkippedResources).To(Equal(0))
			Expect(summary.StableResources).To(Equal(0))
			Expect(summary.RefreshedCount).To(Equal(0))
			Expect(summary.TotalErrors).To(Equal(0))
			Expect(summary.TotalDuration).To(Equal(time.Duration(0)))
		})

		It("Should handle only session start event", func() {
			startEvent := NewSessionStartEvent()
			summary := BuildSessionSummary([]SessionEvent{startEvent})

			Expect(summary.TotalResources).To(Equal(0))
			Expect(summary.StartTime).To(Equal(startEvent.TimeStamp))
		})
	})

	Describe("String", func() {
		It("Should format summary correctly", func() {
			startEvent := NewSessionStartEvent()
			startEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

			changedEvent := NewTransactionEvent("package", "nginx", "")
			changedEvent.Changed = true
			changedEvent.TimeStamp = time.Date(2025, 1, 1, 12, 0, 10, 0, time.UTC)

			summary := BuildSessionSummary([]SessionEvent{startEvent, changedEvent})
			str := summary.String()

			Expect(str).To(ContainSubstring("resources=1"))
			Expect(str).To(ContainSubstring("changed=1"))
			Expect(str).To(ContainSubstring("failed=0"))
			Expect(str).To(ContainSubstring("skipped=0"))
			Expect(str).To(ContainSubstring("stable=0"))
			Expect(str).To(ContainSubstring("duration=10s"))
		})
	})
})
