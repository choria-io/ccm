// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"os"
	"path/filepath"
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

var _ = Describe("Sha256HashBytes", func() {
	It("computes the correct hash for known input", func() {
		// SHA256 of "hello world" is well-known
		hash, err := Sha256HashBytes([]byte("hello world"))
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).To(Equal("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"))
	})

	It("computes the correct hash for empty input", func() {
		// SHA256 of empty string
		hash, err := Sha256HashBytes([]byte{})
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
	})

	It("produces consistent results for the same input", func() {
		input := []byte("test data for hashing")
		hash1, err := Sha256HashBytes(input)
		Expect(err).ToNot(HaveOccurred())
		hash2, err := Sha256HashBytes(input)
		Expect(err).ToNot(HaveOccurred())
		Expect(hash1).To(Equal(hash2))
	})
})

var _ = Describe("Sha256HashFile", func() {
	It("computes the correct hash for a file", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "testfile.txt")
		err := os.WriteFile(testFile, []byte("hello world"), 0644)
		Expect(err).ToNot(HaveOccurred())

		hash, err := Sha256HashFile(testFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).To(Equal("b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"))
	})

	It("computes the correct hash for an empty file", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "emptyfile.txt")
		err := os.WriteFile(testFile, []byte{}, 0644)
		Expect(err).ToNot(HaveOccurred())

		hash, err := Sha256HashFile(testFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).To(Equal("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
	})

	It("returns an error for non-existent file", func() {
		tempDir := GinkgoT().TempDir()
		_, err := Sha256HashFile(filepath.Join(tempDir, "nonexistent.txt"))
		Expect(err).To(HaveOccurred())
	})

	It("produces the same hash as Sha256HashBytes for identical content", func() {
		tempDir := GinkgoT().TempDir()
		content := []byte("matching content between file and bytes")
		testFile := filepath.Join(tempDir, "matchtest.txt")
		err := os.WriteFile(testFile, content, 0644)
		Expect(err).ToNot(HaveOccurred())

		fileHash, err := Sha256HashFile(testFile)
		Expect(err).ToNot(HaveOccurred())

		bytesHash, err := Sha256HashBytes(content)
		Expect(err).ToNot(HaveOccurred())

		Expect(fileHash).To(Equal(bytesHash))
	})
})

var _ = Describe("clone helpers", func() {
	It("clones maps deeply so mutations do not leak", func() {
		// Checks that modifying a cloned map leaves the original untouched.
		source := map[string]any{
			"nested": map[string]any{"value": 1},
			"list":   []any{1, 2},
		}

		cloned := CloneMap(source)
		cloned["nested"].(map[string]any)["value"] = 2
		cloned["list"].([]any)[0] = 99

		Expect(source).To(Equal(map[string]any{
			"nested": map[string]any{"value": 1},
			"list":   []any{1, 2},
		}))
	})

	It("deep merges maps without reusing source slices", func() {
		// Ensures deepMerge concatenates slices while maintaining isolation from inputs.
		target := map[string]any{
			"list": []any{1},
		}
		source := map[string]any{
			"list": []any{2},
		}

		merged := DeepMergeMap(target, source)
		merged["list"].([]any)[0] = 42

		Expect(target["list"].([]any)).To(Equal([]any{1}))
		Expect(source["list"].([]any)).To(Equal([]any{2}))
		Expect(merged["list"].([]any)).To(Equal([]any{42, 2}))
	})
})

var _ = Describe("ShallowMerge", func() {
	It("merges source keys into target", func() {
		target := map[string]any{
			"a": 1,
			"b": 2,
		}
		source := map[string]any{
			"c": 3,
		}

		result := ShallowMerge(target, source)

		Expect(result).To(Equal(map[string]any{
			"a": 1,
			"b": 2,
			"c": 3,
		}))
	})

	It("overwrites target keys with source values", func() {
		target := map[string]any{
			"a": 1,
			"b": 2,
		}
		source := map[string]any{
			"b": 99,
		}

		result := ShallowMerge(target, source)

		Expect(result).To(Equal(map[string]any{
			"a": 1,
			"b": 99,
		}))
	})

	It("does not recursively merge nested maps", func() {
		target := map[string]any{
			"nested": map[string]any{"a": 1, "b": 2},
		}
		source := map[string]any{
			"nested": map[string]any{"b": 99},
		}

		result := ShallowMerge(target, source)

		// Unlike DeepMergeMap, the entire nested map is replaced
		Expect(result["nested"]).To(Equal(map[string]any{"b": 99}))
	})

	It("does not concatenate slices", func() {
		target := map[string]any{
			"list": []any{1, 2},
		}
		source := map[string]any{
			"list": []any{3, 4},
		}

		result := ShallowMerge(target, source)

		// Unlike DeepMergeMap, the slice is replaced not concatenated
		Expect(result["list"]).To(Equal([]any{3, 4}))
	})

	It("does not mutate input maps", func() {
		target := map[string]any{
			"a":      1,
			"nested": map[string]any{"x": 10},
		}
		source := map[string]any{
			"b": 2,
		}

		result := ShallowMerge(target, source)
		result["a"] = 999
		result["nested"].(map[string]any)["x"] = 999

		Expect(target["a"]).To(Equal(1))
		Expect(target["nested"].(map[string]any)["x"]).To(Equal(10))
		Expect(source["b"]).To(Equal(2))
	})

	It("handles empty maps", func() {
		target := map[string]any{}
		source := map[string]any{"a": 1}

		result := ShallowMerge(target, source)
		Expect(result).To(Equal(map[string]any{"a": 1}))

		result2 := ShallowMerge(source, map[string]any{})
		Expect(result2).To(Equal(map[string]any{"a": 1}))
	})
})
