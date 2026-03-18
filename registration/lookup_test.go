// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/synadia-io/orbit.go/jetstreamext"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

var _ = Describe("Registration/JetStreamLookup", func() {
	var (
		mockctl     *gomock.Controller
		mgr         *modelmocks.MockManager
		mockJS      *modelmocks.MockJetStream
		ctx         context.Context
		origGetLast func(context.Context, jetstream.JetStream, string, []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error)
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		mgr = modelmocks.NewMockManager(mockctl)
		mockJS = modelmocks.NewMockJetStream(mockctl)
		ctx = context.Background()

		origGetLast = getLastMsgsFor

		mgr.EXPECT().RegistrationStream().Return("REGISTRATIONS").AnyTimes()
	})

	AfterEach(func() {
		getLastMsgsFor = origGetLast
		mockctl.Finish()
	})

	Describe("filterSubject", func() {
		It("should use all provided values", func() {
			Expect(filterSubject("prod", "tcp", "web", "10.0.0.1")).To(Equal("choria.ccm.registration.v1.prod.tcp.web.10_0_0_1.*"))
		})

		It("should wildcard empty strings", func() {
			Expect(filterSubject("", "", "", "")).To(Equal("choria.ccm.registration.v1.*.*.*.*.*"))
		})

		It("should wildcard star strings", func() {
			Expect(filterSubject("*", "*", "*", "*")).To(Equal("choria.ccm.registration.v1.*.*.*.*.*"))
		})

		It("should handle mixed values and wildcards", func() {
			Expect(filterSubject("prod", "", "web", "*")).To(Equal("choria.ccm.registration.v1.prod.*.web.*.*"))
		})
	})

	Describe("JetStreamLookup", func() {
		It("should return error when JetStream connection fails", func() {
			mgr.EXPECT().JetStream().Return(nil, fmt.Errorf("connection refused"))

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "10.0.0.1")
			Expect(err).To(MatchError(ContainSubstring("could not connect to JetStream")))
			Expect(entries).To(BeNil())
		})

		It("should return error when query fails", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return nil, fmt.Errorf("stream not found")
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "10.0.0.1")
			Expect(err).To(MatchError(ContainSubstring("could not query registrations")))
			Expect(entries).To(BeNil())
		})

		It("should return empty slice when no messages match", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, stream string, subjects []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				Expect(stream).To(Equal("REGISTRATIONS"))
				Expect(subjects).To(Equal([]string{"choria.ccm.registration.v1.prod.tcp.web.10_0_0_1.*"}))

				return func(yield func(*jetstream.RawStreamMsg, error) bool) {}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "10.0.0.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("should deserialize and return entries", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			entry1 := &model.RegistrationEntry{
				Cluster:  "prod",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     int64(8080),
				Priority: 1,
			}
			data1, err := json.Marshal(entry1)
			Expect(err).ToNot(HaveOccurred())

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					yield(&jetstream.RawStreamMsg{Data: data1}, nil)
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Address).To(Equal("10.0.0.1"))
			Expect(entries[0].Port).To(Equal(float64(8080)))
		})

		It("should sort entries by Address then port", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			mkEntry := func(ip string, port int64) []byte {
				e := &model.RegistrationEntry{
					Cluster:  "prod",
					Service:  "web",
					Protocol: "tcp",
					Address:  ip,
					Port:     port,
					Priority: 1,
				}
				d, err := json.Marshal(e)
				Expect(err).ToNot(HaveOccurred())
				return d
			}

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					if !yield(&jetstream.RawStreamMsg{Data: mkEntry("10.0.0.2", 80)}, nil) {
						return
					}
					if !yield(&jetstream.RawStreamMsg{Data: mkEntry("10.0.0.1", 443)}, nil) {
						return
					}
					if !yield(&jetstream.RawStreamMsg{Data: mkEntry("10.0.0.1", 80)}, nil) {
						return
					}
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "*", "*", "*", "*")
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(HaveLen(3))
			Expect(entries[0].Address).To(Equal("10.0.0.1"))
			Expect(entries[0].Port).To(Equal(float64(80)))
			Expect(entries[1].Address).To(Equal("10.0.0.1"))
			Expect(entries[1].Port).To(Equal(float64(443)))
			Expect(entries[2].Address).To(Equal("10.0.0.2"))
			Expect(entries[2].Port).To(Equal(float64(80)))
		})

		It("should return empty list when iterator yields no messages error", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					yield(nil, jetstreamext.ErrNoMessages)
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "10.0.0.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("should return error on other iterator errors", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					yield(nil, fmt.Errorf("stream unavailable"))
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "10.0.0.1")
			Expect(err).To(MatchError(ContainSubstring("could not read registration message")))
			Expect(entries).To(BeNil())
		})

		It("should return error on invalid JSON", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					yield(&jetstream.RawStreamMsg{Data: []byte("{invalid")}, nil)
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "10.0.0.1")
			Expect(err).To(MatchError(ContainSubstring("could not decode registration entry")))
			Expect(entries).To(BeNil())
		})
	})

	Describe("isMarkerMessage", func() {
		DescribeTable("should identify marker messages",
			func(reason string, expected bool) {
				header := nats.Header(http.Header{})
				header.Set("Nats-Marker-Reason", reason)
				msg := &jetstream.RawStreamMsg{Header: header}
				Expect(isMarkerMessage(msg)).To(Equal(expected))
			},
			Entry("MaxAge", "MaxAge", true),
			Entry("Remove", "Remove", true),
			Entry("Purge", "Purge", true),
			Entry("unknown reason", "Other", false),
			Entry("empty reason", "", false),
		)

		It("should return false when no marker header is present", func() {
			msg := &jetstream.RawStreamMsg{}
			Expect(isMarkerMessage(msg)).To(BeFalse())
		})
	})

	Describe("JetStreamLookup with marker messages", func() {
		It("should skip marker messages", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			entry1 := &model.RegistrationEntry{
				Cluster:  "prod",
				Service:  "web",
				Protocol: "tcp",
				Address:  "10.0.0.1",
				Port:     int64(8080),
				Priority: 1,
			}
			data1, err := json.Marshal(entry1)
			Expect(err).ToNot(HaveOccurred())

			markerHeader := nats.Header(http.Header{})
			markerHeader.Set("Nats-Marker-Reason", "MaxAge")

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					if !yield(&jetstream.RawStreamMsg{Data: data1}, nil) {
						return
					}
					yield(&jetstream.RawStreamMsg{Header: markerHeader, Data: []byte("{}")}, nil)
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "*")
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Address).To(Equal("10.0.0.1"))
		})

		It("should return empty list when all messages are markers", func() {
			mgr.EXPECT().JetStream().Return(mockJS, nil)

			markerHeader := nats.Header(http.Header{})
			markerHeader.Set("Nats-Marker-Reason", "Purge")

			getLastMsgsFor = func(_ context.Context, _ jetstream.JetStream, _ string, _ []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
				return func(yield func(*jetstream.RawStreamMsg, error) bool) {
					yield(&jetstream.RawStreamMsg{Header: markerHeader, Data: []byte("{}")}, nil)
				}, nil
			}

			entries, err := JetStreamLookup(ctx, mgr, "prod", "tcp", "web", "*")
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})
	})

	Describe("portInt", func() {
		It("should handle int64", func() {
			Expect(portInt(int64(8080))).To(Equal(int64(8080)))
		})

		It("should handle float64", func() {
			Expect(portInt(float64(8080))).To(Equal(int64(8080)))
		})

		It("should handle json.Number", func() {
			Expect(portInt(json.Number("8080"))).To(Equal(int64(8080)))
		})

		It("should return 0 for nil", func() {
			Expect(portInt(nil)).To(Equal(int64(0)))
		})

		It("should return 0 for string", func() {
			Expect(portInt("not a port")).To(Equal(int64(0)))
		})
	})
})
