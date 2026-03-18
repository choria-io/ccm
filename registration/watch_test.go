// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

var _ = Describe("Registration/Watch", func() {
	var mockctl *gomock.Controller

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("WatchAction", func() {
		It("should have correct string values", func() {
			Expect(Register.String()).To(Equal("register"))
			Expect(Remove.String()).To(Equal("remove"))
			Expect(WatchAction(99).String()).To(Equal("unknown"))
		})
	})

	Describe("parseWatchMessage", func() {
		It("should parse a registration message", func() {
			entry := &model.RegistrationEntry{
				Cluster:  "prod",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     int64(8080),
				Priority: 1,
			}
			data, err := json.Marshal(entry)
			Expect(err).ToNot(HaveOccurred())

			msg := modelmocks.NewMockMsg(mockctl)
			msg.EXPECT().Headers().Return(nats.Header(http.Header{}))
			msg.EXPECT().Data().Return(data)

			event, err := parseWatchMessage(msg)
			Expect(err).ToNot(HaveOccurred())
			Expect(event.Action).To(Equal(Register))
			Expect(event.Entry.Cluster).To(Equal("prod"))
			Expect(event.Entry.Service).To(Equal("web"))
			Expect(event.Entry.Address).To(Equal("10.0.0.1"))
			Expect(event.Reason).To(BeEmpty())
		})

		DescribeTable("should parse marker messages as Remove with entry from subject",
			func(reason string) {
				header := nats.Header(http.Header{})
				header.Set("Nats-Marker-Reason", reason)

				msg := modelmocks.NewMockMsg(mockctl)
				msg.EXPECT().Headers().Return(header)
				msg.EXPECT().Subject().Return("choria.ccm.registration.v1.prod.tcp.web.10_0_0_1.abc123")

				event, err := parseWatchMessage(msg)
				Expect(err).ToNot(HaveOccurred())
				Expect(event.Action).To(Equal(Remove))
				Expect(event.Reason).To(Equal(reason))
				Expect(event.Entry.Cluster).To(Equal("prod"))
				Expect(event.Entry.Protocol).To(Equal("tcp"))
				Expect(event.Entry.Service).To(Equal("web"))
				Expect(event.Entry.Address).To(Equal("10.0.0.1"))
			},
			Entry("MaxAge", "MaxAge"),
			Entry("Remove", "Remove"),
			Entry("Purge", "Purge"),
		)

		It("should handle marker messages with short subject gracefully", func() {
			header := nats.Header(http.Header{})
			header.Set("Nats-Marker-Reason", "MaxAge")

			msg := modelmocks.NewMockMsg(mockctl)
			msg.EXPECT().Headers().Return(header)
			msg.EXPECT().Subject().Return("choria.ccm.registration.v1")

			event, err := parseWatchMessage(msg)
			Expect(err).ToNot(HaveOccurred())
			Expect(event.Action).To(Equal(Remove))
			Expect(event.Entry.Cluster).To(BeEmpty())
		})

		It("should return error on invalid JSON for registration message", func() {
			msg := modelmocks.NewMockMsg(mockctl)
			msg.EXPECT().Headers().Return(nats.Header(http.Header{}))
			msg.EXPECT().Data().Return([]byte("{invalid"))

			_, err := parseWatchMessage(msg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not decode registration entry"))
		})
	})

	Describe("parseSubject", func() {
		It("should parse a valid subject", func() {
			entry := parseSubject("choria.ccm.registration.v1.prod.tcp.web.10_0_0_1.abc123")
			Expect(entry.Cluster).To(Equal("prod"))
			Expect(entry.Protocol).To(Equal("tcp"))
			Expect(entry.Service).To(Equal("web"))
			Expect(entry.Address).To(Equal("10.0.0.1"))
		})

		It("should handle IPv6 addresses without dots", func() {
			entry := parseSubject("choria.ccm.registration.v1.prod.tcp.web.::1.abc123")
			Expect(entry.Cluster).To(Equal("prod"))
			Expect(entry.Address).To(Equal("::1"))
		})

		It("should return empty entry for short subjects", func() {
			entry := parseSubject("choria.ccm.registration.v1")
			Expect(entry.Cluster).To(BeEmpty())
		})
	})

	Describe("JetStreamWatch", func() {
		var (
			mgr    *modelmocks.MockManager
			mockJS *modelmocks.MockJetStream
			ctx    context.Context
		)

		BeforeEach(func() {
			mgr = modelmocks.NewMockManager(mockctl)
			mockJS = modelmocks.NewMockJetStream(mockctl)
			ctx = context.Background()
		})

		It("should return error when JetStream connection fails", func() {
			mgr.EXPECT().JetStream().Return(nil, fmt.Errorf("connection refused"))

			events, err := JetStreamWatch(ctx, mgr, "prod", "tcp", "web", "*")
			Expect(err).To(MatchError(ContainSubstring("could not connect to JetStream")))
			Expect(events).To(BeNil())
		})

		It("should return error when ordered consumer creation fails", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)
			mgr.EXPECT().RegistrationStream().Return("REGISTRATION")
			mockJS.EXPECT().OrderedConsumer(gomock.Any(), "REGISTRATION", gomock.Any()).
				DoAndReturn(func(_ interface{}, stream string, cfg jetstream.OrderedConsumerConfig) (jetstream.Consumer, error) {
					Expect(cfg.DeliverPolicy).To(Equal(jetstream.DeliverLastPerSubjectPolicy))
					Expect(cfg.FilterSubjects).To(Equal([]string{"choria.ccm.registration.v1.*.*.*.*.*"}))
					return nil, fmt.Errorf("stream not found")
				})

			events, err := JetStreamWatch(ctx, mgr, "*", "*", "*", "*")
			Expect(err).To(MatchError(ContainSubstring("could not create ordered consumer")))
			Expect(events).To(BeNil())
		})
	})
})
