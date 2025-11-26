// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPackageutil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal/Util")
}

var _ = Describe("VersionCmp", func() {
	It("compares versions as expected for a simple case", func() {
		Expect(VersionCmp("1.2", "1.3", false)).To(Equal(-1))
	})

	Context("when ignore_trailing_zeroes is true", func() {
		It("equates versions with 2 elements and dots but with unnecessary zero", func() {
			// "10.1.0" vs "10.1"
			Expect(VersionCmp("10.1.0", "10.1", true)).To(Equal(0))
		})

		It("equates versions with 1 element and dot but with unnecessary zero", func() {
			// "11.0" vs "11"
			Expect(VersionCmp("11.0", "11", true)).To(Equal(0))
		})

		It("equates versions with 1 element and dot but with unnecessary zeros", func() {
			// "11.00" vs "11"
			Expect(VersionCmp("11.00", "11", true)).To(Equal(0))
		})

		It("equates versions with dots and irregular zeroes", func() {
			// "11.0.00" vs "11"
			Expect(VersionCmp("11.0.00", "11", true)).To(Equal(0))
		})

		It("equates versions with dashes", func() {
			// "10.1-0" vs "10.1.0-0"
			Expect(VersionCmp("10.1-0", "10.1.0-0", true)).To(Equal(0))
		})

		It("compares versions with dashes after normalization", func() {
			// "10.1-1" vs "10.1.0-0"
			Expect(VersionCmp("10.1-1", "10.1.0-0", true)).To(Equal(1))
		})

		It("does not normalize versions if zeros are not trailing", func() {
			// "1.1" vs "1.0.1"
			Expect(VersionCmp("1.1", "1.0.1", true)).To(Equal(1))
		})
	})

	Context("when ignore_trailing_zeroes is false", func() {
		It("does not equate versions if zeros are not trailing", func() {
			// same as above but with ignoreTrailingZeroes = false
			Expect(VersionCmp("1.1", "1.0.1", false)).To(Equal(1))
		})
	})
})
