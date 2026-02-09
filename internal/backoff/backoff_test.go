// Copyright (c) 2017-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package backoff_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/internal/backoff"
)

func TestBackoff(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal/Backoff")
}

var _ = Describe("Backoff", func() {
	// Fast test policy with very short delays to avoid slow tests
	var fastPolicy backoff.Policy

	BeforeEach(func() {
		fastPolicy = backoff.Policy{
			Millis: []int{1, 2, 3, 4, 5},
		}
	})

	Describe("Duration", func() {
		It("Should return duration within jitter range", func() {
			policy := backoff.Policy{Millis: []int{100}}

			// Run multiple times to verify jitter is applied
			for range 10 {
				d := policy.Duration(0)
				// jitter returns [0.5 * millis .. 1.5 * millis]
				Expect(d).To(BeNumerically(">=", 50*time.Millisecond))
				Expect(d).To(BeNumerically("<=", 150*time.Millisecond))
			}
		})

		It("Should saturate at the last value for n beyond array length", func() {
			policy := backoff.Policy{Millis: []int{10, 20, 30}}

			// n=10 is beyond length 3, should use last value (30)
			for range 10 {
				d := policy.Duration(10)
				// Should be jittered around 30: [15..45]
				Expect(d).To(BeNumerically(">=", 15*time.Millisecond))
				Expect(d).To(BeNumerically("<=", 45*time.Millisecond))
			}
		})

		It("Should return 0 for 0 millis value", func() {
			policy := backoff.Policy{Millis: []int{0, 100}}

			d := policy.Duration(0)
			Expect(d).To(Equal(time.Duration(0)))
		})

		It("Should use correct index for different n values", func() {
			policy := backoff.Policy{Millis: []int{100, 200, 300}}

			// n=1 should use 200ms base
			for range 10 {
				d := policy.Duration(1)
				Expect(d).To(BeNumerically(">=", 100*time.Millisecond))
				Expect(d).To(BeNumerically("<=", 300*time.Millisecond))
			}
		})
	})

	Describe("Sleep", func() {
		It("Should sleep for the specified duration", func() {
			start := time.Now()
			err := fastPolicy.Sleep(context.Background(), 5*time.Millisecond)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically(">=", 5*time.Millisecond))
		})

		It("Should be interrupted by context cancellation", func() {
			ctx, cancel := context.WithCancel(context.Background())

			go func() {
				time.Sleep(5 * time.Millisecond)
				cancel()
			}()

			start := time.Now()
			err := fastPolicy.Sleep(ctx, 1*time.Second)
			elapsed := time.Since(start)

			Expect(err).To(Equal(context.Canceled))
			Expect(elapsed).To(BeNumerically("<", 100*time.Millisecond))
		})

		It("Should return immediately if context is already canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			start := time.Now()
			err := fastPolicy.Sleep(ctx, 1*time.Second)
			elapsed := time.Since(start)

			Expect(err).To(Equal(context.Canceled))
			Expect(elapsed).To(BeNumerically("<", 10*time.Millisecond))
		})
	})

	Describe("TrySleep", func() {
		It("Should sleep based on try number", func() {
			policy := backoff.Policy{Millis: []int{5, 10, 15}}

			start := time.Now()
			err := policy.TrySleep(context.Background(), 0)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			// First try uses 5ms, with jitter [2.5..7.5]ms
			Expect(elapsed).To(BeNumerically(">=", 2*time.Millisecond))
		})

		It("Should be interrupted by context", func() {
			ctx, cancel := context.WithCancel(context.Background())
			// Use a policy with longer delays to ensure cancellation happens during sleep
			slowPolicy := backoff.Policy{Millis: []int{100, 200, 300}}

			go func() {
				time.Sleep(5 * time.Millisecond)
				cancel()
			}()

			err := slowPolicy.TrySleep(ctx, 0)
			Expect(err).To(Equal(context.Canceled))
		})
	})

	Describe("For", func() {
		It("Should stop when callback returns nil", func() {
			attempts := 0

			err := fastPolicy.For(context.Background(), func(try int) error {
				attempts++
				if attempts >= 3 {
					return nil
				}
				return errors.New("not yet")
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(attempts).To(Equal(3))
		})

		It("Should pass incrementing try number to callback", func() {
			var tries []int

			err := fastPolicy.For(context.Background(), func(try int) error {
				tries = append(tries, try)
				if len(tries) >= 4 {
					return nil
				}
				return errors.New("continue")
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tries).To(Equal([]int{1, 2, 3, 4}))
		})

		It("Should stop when context is canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			attempts := 0

			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()

			err := fastPolicy.For(ctx, func(try int) error {
				attempts++
				return errors.New("keep going")
			})

			Expect(err).To(Equal(context.Canceled))
			Expect(attempts).To(BeNumerically(">=", 1))
		})

		It("Should return immediately if context is already canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			attempts := 0
			err := fastPolicy.For(ctx, func(try int) error {
				attempts++
				return errors.New("should not reach here")
			})

			Expect(err).To(Equal(context.Canceled))
			Expect(attempts).To(Equal(0))
		})

		It("Should succeed on first try if callback returns nil immediately", func() {
			attempts := 0

			err := fastPolicy.For(context.Background(), func(try int) error {
				attempts++
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(attempts).To(Equal(1))
		})
	})

	Describe("AfterFunc", func() {
		It("Should call function after duration", func() {
			called := make(chan bool, 1)

			fastPolicy.AfterFunc(0, func() {
				called <- true
			})

			select {
			case <-called:
				// Success
			case <-time.After(50 * time.Millisecond):
				Fail("AfterFunc did not call function in time")
			}
		})
	})

	Describe("Predefined policies", func() {
		It("Should have FiveSecStartGrace with initial grace period", func() {
			Expect(backoff.FiveSecStartGrace.Millis[0]).To(Equal(0))
			Expect(backoff.FiveSecStartGrace.Millis[len(backoff.FiveSecStartGrace.Millis)-1]).To(Equal(5000))
		})

		It("Should have FiveSec starting at 500ms", func() {
			Expect(backoff.FiveSec.Millis[0]).To(Equal(500))
			Expect(backoff.FiveSec.Millis[len(backoff.FiveSec.Millis)-1]).To(Equal(5000))
		})

		It("Should have TwentySec going up to 20 seconds", func() {
			Expect(backoff.TwentySec.Millis[0]).To(Equal(500))
			Expect(backoff.TwentySec.Millis[len(backoff.TwentySec.Millis)-1]).To(Equal(20000))
		})

		It("Should have Default set to TwentySec", func() {
			Expect(backoff.Default.Millis).To(Equal(backoff.TwentySec.Millis))
		})
	})

	Describe("InterruptableSleep", func() {
		It("Should sleep for the specified duration", func() {
			start := time.Now()
			err := backoff.InterruptableSleep(context.Background(), 5*time.Millisecond)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically(">=", 5*time.Millisecond))
		})

		It("Should return nil immediately for zero duration", func() {
			start := time.Now()
			err := backoff.InterruptableSleep(context.Background(), 0)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 10*time.Millisecond))
		})

		It("Should be interrupted by context cancellation", func() {
			ctx, cancel := context.WithCancel(context.Background())

			go func() {
				time.Sleep(5 * time.Millisecond)
				cancel()
			}()

			start := time.Now()
			err := backoff.InterruptableSleep(ctx, 1*time.Second)
			elapsed := time.Since(start)

			Expect(err).To(MatchError("sleep interrupted by context"))
			Expect(elapsed).To(BeNumerically("<", 100*time.Millisecond))
		})

		It("Should return error immediately if context is already canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			start := time.Now()
			err := backoff.InterruptableSleep(ctx, 1*time.Second)
			elapsed := time.Since(start)

			Expect(err).To(MatchError("sleep interrupted by context"))
			Expect(elapsed).To(BeNumerically("<", 10*time.Millisecond))
		})
	})

	Describe("jitter", func() {
		It("Should return values in expected range", func() {
			// Test indirectly through Duration since jitter is unexported
			policy := backoff.Policy{Millis: []int{1000}}

			minSeen := time.Hour
			maxSeen := time.Duration(0)

			for range 100 {
				d := policy.Duration(0)
				if d < minSeen {
					minSeen = d
				}
				if d > maxSeen {
					maxSeen = d
				}
			}

			// With 100 samples, we should see reasonable spread
			// Expected range is [500ms, 1500ms]
			Expect(minSeen).To(BeNumerically(">=", 500*time.Millisecond))
			Expect(maxSeen).To(BeNumerically("<=", 1500*time.Millisecond))
			// Should have some spread (not all the same value)
			Expect(maxSeen - minSeen).To(BeNumerically(">", 100*time.Millisecond))
		})
	})
})
