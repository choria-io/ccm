// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"testing"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}

var _ = Describe("WithNatsContext", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets the NATS context on the manager", func() {
		mgr, err := NewManager(mockLog, mockLog, WithNatsContext("test-context"))
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.natsContext).To(Equal("test-context"))
	})

	It("allows empty context to be set", func() {
		mgr, err := NewManager(mockLog, mockLog, WithNatsContext(""))
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.natsContext).To(Equal(""))
	})
})

var _ = Describe("JetStream", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mockJS  *modelmocks.MockJetStream
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockJS = modelmocks.NewMockJetStream(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns an error when NATS context is not set", func() {
		mgr, err := NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())

		js, err := mgr.JetStream()
		Expect(err).To(MatchError("nats context not set"))
		Expect(js).To(BeNil())
	})

	It("returns cached JetStream when already initialized", func() {
		mgr, err := NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())

		// Pre-populate the js field to simulate a cached connection
		mgr.js = mockJS

		js, err := mgr.JetStream()
		Expect(err).NotTo(HaveOccurred())
		Expect(js).To(Equal(mockJS))

		// Call again to verify caching
		js2, err := mgr.JetStream()
		Expect(err).NotTo(HaveOccurred())
		Expect(js2).To(Equal(mockJS))
	})

	It("returns cached JetStream even when context is set", func() {
		mgr, err := NewManager(mockLog, mockLog, WithNatsContext("some-context"))
		Expect(err).NotTo(HaveOccurred())

		// Pre-populate the js field - should return this instead of trying to connect
		mgr.js = mockJS

		js, err := mgr.JetStream()
		Expect(err).NotTo(HaveOccurred())
		Expect(js).To(Equal(mockJS))
	})
})

var _ = Describe("WithNoop", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets noop mode on the manager", func() {
		mgr, err := NewManager(mockLog, mockLog, WithNoop())
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.NoopMode()).To(BeTrue())
	})

	It("defaults to noop mode being false", func() {
		mgr, err := NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.NoopMode()).To(BeFalse())
	})
})

var _ = Describe("WithEnvironmentData", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets environment data on the manager", func() {
		envData := map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		}

		mgr, err := NewManager(mockLog, mockLog, WithEnvironmentData(envData))
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.Environment()).To(Equal(envData))
	})

	It("handles nil environment data", func() {
		mgr, err := NewManager(mockLog, mockLog, WithEnvironmentData(nil))
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.Environment()).To(Equal(map[string]string{}))
	})
})

var _ = Describe("NewManager", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("creates a manager with default session store", func() {
		mgr, err := NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr).NotTo(BeNil())
		Expect(mgr.session).NotTo(BeNil())
	})

	It("applies multiple options", func() {
		envData := map[string]string{"KEY": "value"}
		mgr, err := NewManager(mockLog, mockLog,
			WithNatsContext("my-context"),
			WithNoop(),
			WithEnvironmentData(envData),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(mgr.natsContext).To(Equal("my-context"))
		Expect(mgr.NoopMode()).To(BeTrue())
		Expect(mgr.Environment()).To(Equal(envData))
	})
})

var _ = Describe("SetExternalData", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets external data on the manager", func() {
		extData := map[string]any{
			"external_key": "external_value",
			"nested": map[string]any{
				"key": "value",
			},
		}

		mgr.SetExternalData(extData)
		Expect(mgr.externData).To(Equal(extData))
	})

	It("handles nil external data", func() {
		mgr.SetExternalData(nil)
		Expect(mgr.externData).To(BeNil())
	})
})

var _ = Describe("WorkingDirectory", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets and gets the working directory", func() {
		mgr.SetWorkingDirectory("/tmp/test-dir")
		Expect(mgr.WorkingDirectory()).To(Equal("/tmp/test-dir"))
	})

	It("returns empty string when not set", func() {
		Expect(mgr.WorkingDirectory()).To(Equal(""))
	})
})

var _ = Describe("Data", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets and returns data", func() {
		data := map[string]any{
			"key1": "value1",
			"key2": 42,
		}

		result := mgr.SetData(data)
		Expect(result).To(Equal(data))
		Expect(mgr.Data()).To(Equal(data))
	})

	It("merges data with external data", func() {
		extData := map[string]any{
			"external": "value",
			"key1":     "overridden",
		}
		mgr.SetExternalData(extData)

		data := map[string]any{
			"key1": "original",
			"key2": "value2",
		}

		result := mgr.SetData(data)

		// External data should override the provided data
		Expect(result["external"]).To(Equal("value"))
		Expect(result["key1"]).To(Equal("overridden"))
		Expect(result["key2"]).To(Equal("value2"))
	})

	It("returns a copy of the data", func() {
		data := map[string]any{"key": "value"}
		mgr.SetData(data)

		retrieved := mgr.Data()
		retrieved["new_key"] = "new_value"

		// Original should not be affected
		Expect(mgr.Data()).NotTo(HaveKey("new_key"))
	})

	It("returns empty map when no data is set", func() {
		Expect(mgr.Data()).To(Equal(map[string]any{}))
	})
})

var _ = Describe("Facts", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		ctx = context.Background()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns facts set with SetFacts", func() {
		facts := map[string]any{
			"hostname": "test-host",
			"os":       "linux",
		}

		mgr.SetFacts(facts)

		result, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(facts))
	})

	It("returns cached facts on subsequent calls", func() {
		facts := map[string]any{"cached": true}
		mgr.SetFacts(facts)

		result1, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())

		result2, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result1).To(Equal(result2))
	})

	It("gathers and caches facts when not pre-set", func() {
		// Without calling SetFacts, Facts() should gather system facts
		result, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeNil())

		// Subsequent calls should return cached facts (same map)
		result2, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result2).To(Equal(result))
	})

	It("SetFacts updates the cached facts", func() {
		// First set some facts
		initialFacts := map[string]any{"initial": "value"}
		mgr.SetFacts(initialFacts)

		result1, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result1).To(Equal(initialFacts))

		// Update with new facts
		updatedFacts := map[string]any{"updated": "data"}
		mgr.SetFacts(updatedFacts)

		result2, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result2).To(Equal(updatedFacts))
		Expect(result2).NotTo(Equal(initialFacts))
	})
})

var _ = Describe("MergeFacts", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		ctx = context.Background()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("merges provided facts with cached facts", func() {
		// Set base facts
		baseFacts := map[string]any{
			"hostname": "test-host",
			"os":       "linux",
		}
		mgr.SetFacts(baseFacts)

		// Merge additional facts
		additionalFacts := map[string]any{
			"custom_key": "custom_value",
		}

		result, err := mgr.MergeFacts(ctx, additionalFacts)
		Expect(err).NotTo(HaveOccurred())

		// Result should contain both base and additional facts
		Expect(result["hostname"]).To(Equal("test-host"))
		Expect(result["os"]).To(Equal("linux"))
		Expect(result["custom_key"]).To(Equal("custom_value"))
	})

	It("provided facts override system facts with same key", func() {
		// Set base facts (these are the "system" facts)
		baseFacts := map[string]any{
			"hostname": "system-host",
			"version":  "1.0",
		}
		mgr.SetFacts(baseFacts)

		// Merge facts with conflicting key
		providedFacts := map[string]any{
			"hostname": "user-host", // This should override system facts
			"custom":   "value",
		}

		result, err := mgr.MergeFacts(ctx, providedFacts)
		Expect(err).NotTo(HaveOccurred())

		// Provided facts override system facts
		Expect(result["hostname"]).To(Equal("user-host"))
		Expect(result["version"]).To(Equal("1.0"))
		Expect(result["custom"]).To(Equal("value"))
	})

	It("updates the internal facts cache", func() {
		baseFacts := map[string]any{"base": "value"}
		mgr.SetFacts(baseFacts)

		additionalFacts := map[string]any{"additional": "data"}
		_, err := mgr.MergeFacts(ctx, additionalFacts)
		Expect(err).NotTo(HaveOccurred())

		// Subsequent Facts() call should return the merged result
		result, err := mgr.Facts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result["base"]).To(Equal("value"))
		Expect(result["additional"]).To(Equal("data"))
	})

	It("deep merges nested maps", func() {
		baseFacts := map[string]any{
			"nested": map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		}
		mgr.SetFacts(baseFacts)

		additionalFacts := map[string]any{
			"nested": map[string]any{
				"key3": "value3",
			},
		}

		result, err := mgr.MergeFacts(ctx, additionalFacts)
		Expect(err).NotTo(HaveOccurred())

		nested := result["nested"].(map[string]any)
		Expect(nested["key1"]).To(Equal("value1"))
		Expect(nested["key2"]).To(Equal("value2"))
		Expect(nested["key3"]).To(Equal("value3"))
	})
})

var _ = Describe("FactsRaw", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		ctx = context.Background()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns facts as JSON", func() {
		facts := map[string]any{
			"hostname": "test-host",
			"count":    float64(42),
		}
		mgr.SetFacts(facts)

		raw, err := mgr.FactsRaw(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(raw).To(MatchJSON(`{"hostname":"test-host","count":42}`))
	})
})

var _ = Describe("Logger", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("creates a logger with key-value pairs", func() {
		log, err := mgr.Logger("component", "test", "id", 123)
		Expect(err).NotTo(HaveOccurred())
		Expect(log).NotTo(BeNil())
	})

	It("returns an error for odd number of arguments", func() {
		_, err := mgr.Logger("component", "test", "orphan")
		Expect(err).To(MatchError("invalid logger arguments, must be key value pairs"))
	})

	It("accepts no arguments", func() {
		log, err := mgr.Logger()
		Expect(err).NotTo(HaveOccurred())
		Expect(log).NotTo(BeNil())
	})
})

var _ = Describe("NewRunner", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("creates a command runner", func() {
		runner, err := mgr.NewRunner()
		Expect(err).NotTo(HaveOccurred())
		Expect(runner).NotTo(BeNil())
	})
})

var _ = Describe("RecordEvent", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("records an event in the session store", func() {
		event := &model.TransactionEvent{
			ResourceType: "file",
			Name:         "/tmp/test",
			Changed:      true,
		}

		err := mgr.RecordEvent(event)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when session store is nil", func() {
		mgr.session = nil

		event := &model.TransactionEvent{}
		err := mgr.RecordEvent(event)
		Expect(err).To(MatchError("no session store available"))
	})

	It("returns an error when event name is empty", func() {
		event := &model.TransactionEvent{
			ResourceType: "file",
			Name:         "",
		}

		err := mgr.RecordEvent(event)
		Expect(err).To(MatchError("event name cannot be empty"))
	})

	It("returns an error when resource type is empty", func() {
		event := &model.TransactionEvent{
			ResourceType: "",
			Name:         "/tmp/test",
		}

		err := mgr.RecordEvent(event)
		Expect(err).To(MatchError("resource type cannot be empty"))
	})
})

var _ = Describe("ShouldRefresh", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns true when the last event was changed", func() {
		event := &model.TransactionEvent{
			ResourceType: "file",
			Name:         "/tmp/test",
			Changed:      true,
		}
		err := mgr.RecordEvent(event)
		Expect(err).NotTo(HaveOccurred())

		shouldRefresh, err := mgr.ShouldRefresh("file", "/tmp/test")
		Expect(err).NotTo(HaveOccurred())
		Expect(shouldRefresh).To(BeTrue())
	})

	It("returns false when the last event was not changed", func() {
		event := &model.TransactionEvent{
			ResourceType: "service",
			Name:         "nginx",
			Changed:      false,
		}
		err := mgr.RecordEvent(event)
		Expect(err).NotTo(HaveOccurred())

		shouldRefresh, err := mgr.ShouldRefresh("service", "nginx")
		Expect(err).NotTo(HaveOccurred())
		Expect(shouldRefresh).To(BeFalse())
	})

	It("returns an error when no events exist for the resource", func() {
		_, err := mgr.ShouldRefresh("file", "/nonexistent")
		Expect(err).To(MatchError("no events found for file#/nonexistent"))
	})

	It("returns an error when session store is nil", func() {
		mgr.session = nil

		_, err := mgr.ShouldRefresh("file", "/tmp/test")
		Expect(err).To(MatchError("no session store available"))
	})
})

var _ = Describe("SessionSummary", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns a session summary", func() {
		// Record some events
		err := mgr.RecordEvent(&model.TransactionEvent{
			ResourceType: "file",
			Name:         "/tmp/test1",
			Changed:      true,
		})
		Expect(err).NotTo(HaveOccurred())

		err = mgr.RecordEvent(&model.TransactionEvent{
			ResourceType: "file",
			Name:         "/tmp/test2",
			Changed:      false,
		})
		Expect(err).NotTo(HaveOccurred())

		summary, err := mgr.SessionSummary()
		Expect(err).NotTo(HaveOccurred())
		Expect(summary).NotTo(BeNil())
		Expect(summary.TotalResources).To(Equal(2))
		Expect(summary.ChangedResources).To(Equal(1))
		Expect(summary.StableResources).To(Equal(1))
	})

	It("returns an error when session store is nil", func() {
		mgr.session = nil

		_, err := mgr.SessionSummary()
		Expect(err).To(MatchError("no session store available"))
	})
})

var _ = Describe("TemplateEnvironment", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		ctx = context.Background()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns the template environment with all data", func() {
		facts := map[string]any{"hostname": "test-host"}
		data := map[string]any{"app": "myapp"}
		env := map[string]string{"ENV_VAR": "value"}

		mgr.SetFacts(facts)
		mgr.SetData(data)
		mgr.SetEnviron(env)
		mgr.SetWorkingDirectory("/tmp/working")

		tplEnv, err := mgr.TemplateEnvironment(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(tplEnv.Facts).To(Equal(facts))
		Expect(tplEnv.Data).To(Equal(data))
		Expect(tplEnv.Environ).To(Equal(env))
		Expect(tplEnv.WorkingDir).To(Equal("/tmp/working"))
	})
})

var _ = Describe("StartSession", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns the session store", func() {
		// StartSession requires a valid Apply, but we can verify
		// that the session store is properly initialized
		Expect(mgr.session).NotTo(BeNil())
	})
})

var _ = Describe("ResourceInfo", func() {
	var (
		ctrl    *gomock.Controller
		mockLog *modelmocks.MockLogger
		mgr     *CCM
		ctx     context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLog = modelmocks.NewMockLogger(ctrl)
		mockLog.EXPECT().With(gomock.Any()).AnyTimes().Return(mockLog)
		mockLog.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockLog.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		ctx = context.Background()

		var err error
		mgr, err = NewManager(mockLog, mockLog)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("returns an error for unsupported resource type", func() {
		_, err := mgr.ResourceInfo(ctx, "unsupported", "test")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown resource type unsupported"))
	})
})

// Verify JetStream interface compliance
var _ jetstream.JetStream = (*modelmocks.MockJetStream)(nil)
