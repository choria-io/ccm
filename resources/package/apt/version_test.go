// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apt

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Debian Version", func() {
	Describe("ParseVersion", func() {
		It("Should reject empty strings", func() {
			_, err := ParseVersion("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty string"))
		})

		It("Should reject invalid version strings", func() {
			_, err := ParseVersion("!!!invalid!!!")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to parse"))
		})

		DescribeTable("Should parse valid version strings",
			func(input string, expectedEpoch int, expectedUpstream string, expectedRevision string) {
				v, err := ParseVersion(input)
				Expect(err).ToNot(HaveOccurred())
				Expect(v.Epoch).To(Equal(expectedEpoch))
				Expect(v.UpstreamVersion).To(Equal(expectedUpstream))
				Expect(v.DebianRevision).To(Equal(expectedRevision))
			},
			Entry("version with epoch and revision",
				"1:20191210.1-0ubuntu0.19.04.2", 1, "20191210.1", "0ubuntu0.19.04.2"),
			Entry("version without epoch",
				"20191210.1-0ubuntu0.19.04.2", 0, "20191210.1", "0ubuntu0.19.04.2"),
			Entry("version with plus sign, no revision",
				"2.42.1+19.04", 0, "2.42.1+19.04", ""),
			Entry("version with git suffix and tilde in revision",
				"3.32.2+git20190711-2ubuntu1~19.04.1", 0, "3.32.2+git20190711", "2ubuntu1~19.04.1"),
			Entry("version with high epoch and complex upstream",
				"5:1.0.0+git-20190109.133f4c4-0ubuntu2", 5, "1.0.0+git-20190109.133f4c4", "0ubuntu2"),
			Entry("simple version",
				"1.0", 0, "1.0", ""),
			Entry("version with only revision",
				"1.0-1", 0, "1.0", "1"),
			Entry("version with tilde",
				"1.0~beta1", 0, "1.0~beta1", ""),
		)
	})

	Describe("String", func() {
		It("Should format version without epoch correctly", func() {
			v := &Version{Epoch: 0, UpstreamVersion: "1.0", DebianRevision: "1"}
			Expect(v.String()).To(Equal("1.0-1"))
		})

		It("Should format version with epoch correctly", func() {
			v := &Version{Epoch: 2, UpstreamVersion: "1.0", DebianRevision: "1"}
			Expect(v.String()).To(Equal("2:1.0-1"))
		})

		It("Should format version without revision correctly", func() {
			v := &Version{Epoch: 0, UpstreamVersion: "1.0", DebianRevision: ""}
			Expect(v.String()).To(Equal("1.0"))
		})

		It("Should round-trip parsed versions", func() {
			inputs := []string{
				"1:20191210.1-0ubuntu0.19.04.2",
				"20191210.1-0ubuntu0.19.04.2",
				"2.42.1+19.04",
				"3.32.2+git20190711-2ubuntu1~19.04.1",
				"5:1.0.0+git-20190109.133f4c4-0ubuntu2",
			}
			for _, input := range inputs {
				v, err := ParseVersion(input)
				Expect(err).ToNot(HaveOccurred())
				Expect(v.String()).To(Equal(input))
			}
		})
	})

	Describe("Compare", func() {
		DescribeTable("Should compare versions correctly",
			func(lower string, higher string) {
				vLower, err := ParseVersion(lower)
				Expect(err).ToNot(HaveOccurred())

				vHigher, err := ParseVersion(higher)
				Expect(err).ToNot(HaveOccurred())

				Expect(vLower.Compare(vHigher)).To(Equal(-1), "%s should be less than %s", lower, higher)
				Expect(vHigher.Compare(vLower)).To(Equal(1), "%s should be greater than %s", higher, lower)
				Expect(vLower.LessThan(vHigher)).To(BeTrue())
				Expect(vHigher.GreaterThan(vLower)).To(BeTrue())
			},
			Entry("epoch takes precedence", "9:99-99", "10:01-01"),
			Entry("longer revision is greater", "abd-de", "abd-def"),
			Entry("shorter string is smaller", "a1b2d-d3e", "a1b2d-d3ef"),
			Entry("numeric comparison in revision", "a1b2d-d9", "a1b2d-d13"),
			Entry("tilde sorts before empty", "a1b2d-d10~", "a1b2d-d10"),
			Entry("letters sort before non-letters", "a1b2d-d1a", "a1b2d-d1-"),
			Entry("simple numeric versions", "1.0", "2.0"),
			Entry("patch version comparison", "1.0.1", "1.0.2"),
			Entry("minor version comparison", "1.1", "1.2"),
			Entry("revision comparison", "1.0-1", "1.0-2"),
			Entry("tilde prerelease", "1.0~alpha", "1.0"),
			Entry("tilde ordering", "1.0~~", "1.0~"),
			Entry("tilde with suffix", "1.0~a", "1.0"),
			Entry("multiple tildes", "1.0~~a", "1.0~a"),
		)

		It("Should return 0 for equal versions", func() {
			v1, err := ParseVersion("abd-def")
			Expect(err).ToNot(HaveOccurred())

			v2, err := ParseVersion("abd-def")
			Expect(err).ToNot(HaveOccurred())

			Expect(v1.Compare(v2)).To(Equal(0))
			Expect(v1.Equal(v2)).To(BeTrue())
		})

		It("Should handle nil comparison", func() {
			v, err := ParseVersion("1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(v.Compare(nil)).To(Equal(1))
			Expect(v.Equal(nil)).To(BeFalse())
		})
	})

	Describe("CompareVersionStrings", func() {
		It("Should compare valid version strings", func() {
			cmp, err := CompareVersionStrings("1.0", "2.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(-1))

			cmp, err = CompareVersionStrings("2.0", "1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(1))

			cmp, err = CompareVersionStrings("1.0", "1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(cmp).To(Equal(0))
		})

		It("Should return error for invalid first version", func() {
			_, err := CompareVersionStrings("!!!invalid!!!", "1.0")
			Expect(err).To(HaveOccurred())
		})

		It("Should return error for invalid second version", func() {
			_, err := CompareVersionStrings("1.0", "!!!invalid!!!")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Comparison helper methods", func() {
		var v1, v2 *Version

		BeforeEach(func() {
			var err error
			v1, err = ParseVersion("1.0")
			Expect(err).ToNot(HaveOccurred())
			v2, err = ParseVersion("2.0")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should implement LessThanOrEqual", func() {
			Expect(v1.LessThanOrEqual(v2)).To(BeTrue())
			Expect(v1.LessThanOrEqual(v1)).To(BeTrue())
			Expect(v2.LessThanOrEqual(v1)).To(BeFalse())
		})

		It("Should implement GreaterThanOrEqual", func() {
			Expect(v2.GreaterThanOrEqual(v1)).To(BeTrue())
			Expect(v2.GreaterThanOrEqual(v2)).To(BeTrue())
			Expect(v1.GreaterThanOrEqual(v2)).To(BeFalse())
		})
	})
})
