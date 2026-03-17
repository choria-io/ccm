// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

var _ = Describe("Registration/JetStreamPublisher", func() {
	var (
		mockctl *gomock.Controller
		logger  *modelmocks.MockLogger
		mockJS  *MockjetStreamMessagePublisher
		ctx     context.Context
		entry   *model.RegistrationEntry
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		mockJS = NewMockjetStreamMessagePublisher(mockctl)
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
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("newJetStreamPublisher", func() {
		It("should return error when connection is nil", func() {
			_, err := newJetStreamPublisher(nil, "", logger)
			Expect(err).To(MatchError("no nats connection provided"))
		})
	})

	Describe("Publish", func() {
		It("should delegate to NatsPublisher", func() {
			pub := &jetStreamPublisher{
				nats: &natsPublisher{
					log:      logger,
					js:       mockJS,
					reliable: true,
					stream:   "REGISTRATIONS",
				},
			}

			expectedSubject := fmt.Sprintf("ccm.registration.v1.prod.tcp.web.10_0_0_1.%s", entry.InstanceId())
			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
					Expect(msg.Subject).To(Equal(expectedSubject))
					return &jetstream.PubAck{Stream: "REG", Sequence: 1}, nil
				},
			)

			err := pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should set reliable flag on the underlying NatsPublisher", func() {
			pub := &jetStreamPublisher{
				nats: &natsPublisher{
					log:      logger,
					reliable: true,
					js:       mockJS,
				},
			}

			Expect(pub.nats.reliable).To(BeTrue())

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).Return(&jetstream.PubAck{Stream: "REG", Sequence: 1}, nil)

			err := pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should set expected stream header when stream is configured", func() {
			pub := &jetStreamPublisher{
				nats: &natsPublisher{
					log:      logger,
					js:       mockJS,
					reliable: true,
					stream:   "REGISTRATIONS",
				},
			}

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, msg *nats.Msg, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
					Expect(msg.Header.Get(natsExpectedStreamHeader)).To(Equal("REGISTRATIONS"))
					return &jetstream.PubAck{Stream: "REGISTRATIONS", Sequence: 1}, nil
				},
			)

			err := pub.Publish(ctx, entry)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should propagate publish errors", func() {
			pub := &jetStreamPublisher{
				nats: &natsPublisher{
					log:      logger,
					js:       mockJS,
					reliable: true,
				},
			}

			mockJS.EXPECT().PublishMsg(ctx, gomock.Any()).Return(nil, fmt.Errorf("timeout"))

			err := pub.Publish(ctx, entry)
			Expect(err).To(MatchError("timeout"))
		})
	})

	Describe("RegistrationPublisher interface", func() {
		It("should satisfy the RegistrationPublisher interface", func() {
			pub := &jetStreamPublisher{
				nats: &natsPublisher{
					log: logger,
					js:  mockJS,
				},
			}

			var _ model.RegistrationPublisher = pub
			_ = pub
		})
	})
})
