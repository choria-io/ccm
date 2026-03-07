// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package regpublisher

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestNatsPublisher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NatsPublisher")
}

var _ = Describe("NatsPublisher", func() {
	var (
		mockctl    *gomock.Controller
		logger     *modelmocks.MockLogger
		mockJS     *MockJetStreamMessagePublisher
		ctx        context.Context
		entry      *model.RegistrationEntry
		instanceID string
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		mockJS = NewMockJetStreamMessagePublisher(mockctl)
		ctx = context.Background()

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

		entry = &model.RegistrationEntry{
			Cluster:  "prod",
			Service:  "web",
			Protocol: "tcp",
			IP:       "10.0.0.1",
			Port:     int64(8080),
			Priority: 1,
		}
		instanceID = entry.InstanceId()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("natsSubject", func() {
		It("should construct the correct subject", func() {
			p := &NatsPublisher{}
			subject := p.natsSubject(entry, instanceID)
			Expect(subject).To(Equal(fmt.Sprintf("ccm.registration.v1.prod.tcp.web.10.0.0.1.%s", instanceID)))
		})
	})

	Describe("message", func() {
		It("should create a message with JSON body", func() {
			p := &NatsPublisher{}
			msg, err := p.message(entry)
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Subject).To(Equal(fmt.Sprintf("ccm.registration.v1.prod.tcp.web.10.0.0.1.%s", instanceID)))

			var decoded model.RegistrationEntry
			err = json.Unmarshal(msg.Data, &decoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(decoded.Cluster).To(Equal("prod"))
			Expect(decoded.Service).To(Equal("web"))
			Expect(decoded.Protocol).To(Equal("tcp"))
			Expect(decoded.IP).To(Equal("10.0.0.1"))
			Expect(decoded.Port).To(Equal(float64(8080)))
		})

		It("should set TTL and rollup headers when reliable and TTL is set", func() {
			entry.TTL = 30 * time.Second
			p := &NatsPublisher{reliable: true}
			msg, err := p.message(entry)
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Header.Get(natsTTLHeader)).To(Equal("30s"))
			Expect(msg.Header.Get(natsRollupHeader)).To(Equal(natsSubRollup))
		})

		It("should set rollup header but not TTL header when reliable and TTL is zero", func() {
			p := &NatsPublisher{reliable: true}
			msg, err := p.message(entry)
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Header.Get(natsTTLHeader)).To(BeEmpty())
			Expect(msg.Header.Get(natsRollupHeader)).To(Equal(natsSubRollup))
		})

		It("should not set TTL or rollup headers when not reliable", func() {
			p := &NatsPublisher{}
			msg, err := p.message(entry)
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.Header.Get(natsTTLHeader)).To(BeEmpty())
			Expect(msg.Header.Get(natsRollupHeader)).To(BeEmpty())
		})
	})

	Describe("Publish", func() {
		It("should publish via JetStream", func() {
			pub := &NatsPublisher{
				log:      logger,
				js:       mockJS,
				reliable: true,
			}

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
					Expect(msg.Subject).To(Equal(fmt.Sprintf("ccm.registration.v1.prod.tcp.web.10.0.0.1.%s", instanceID)))
					return &jetstream.PubAck{Stream: "REG", Sequence: 42}, nil
				},
			)

			err := pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create JetStream via factory on first publish", func() {
			pub := &NatsPublisher{
				log:      logger,
				reliable: true,
				jsFactory: func(_ *nats.Conn) (JetStreamMessagePublisher, error) {
					return mockJS, nil
				},
			}

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).Return(&jetstream.PubAck{Stream: "REG", Sequence: 1}, nil)

			err := pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when JetStream factory fails", func() {
			pub := &NatsPublisher{
				log:      logger,
				reliable: true,
				jsFactory: func(_ *nats.Conn) (JetStreamMessagePublisher, error) {
					return nil, fmt.Errorf("connection refused")
				},
			}

			err := pub.Publish(ctx, entry)
			Expect(err).To(MatchError("connection refused"))
		})

		It("should return error when JetStream publish fails", func() {
			pub := &NatsPublisher{
				log:      logger,
				js:       mockJS,
				reliable: true,
			}

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).Return(nil, fmt.Errorf("stream not found"))

			err := pub.Publish(ctx, entry)
			Expect(err).To(MatchError("stream not found"))
		})

		It("should reuse JetStream connection on subsequent publishes", func() {
			callCount := 0
			pub := &NatsPublisher{
				log:      logger,
				reliable: true,
				jsFactory: func(_ *nats.Conn) (JetStreamMessagePublisher, error) {
					callCount++
					return mockJS, nil
				},
			}

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).Return(&jetstream.PubAck{Stream: "REG", Sequence: 1}, nil).Times(2)

			err := pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())
			err = pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())

			Expect(callCount).To(Equal(1))
		})
	})

	Describe("NewNatsPublisher", func() {
		It("should return error when connection is nil", func() {
			_, err := NewNatsPublisher(nil, logger)
			Expect(err).To(MatchError("no nats connection provided"))
		})
	})
})
