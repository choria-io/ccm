// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

var _ = Describe("MapStringsToMapStringAny", func() {
	It("converts map[string]string to map[string]any", func() {
		input := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result := MapStringsToMapStringAny(input)

		Expect(result).To(Equal(map[string]any{
			"key1": "value1",
			"key2": "value2",
		}))
	})

	It("handles empty map", func() {
		input := map[string]string{}

		result := MapStringsToMapStringAny(input)

		Expect(result).To(BeEmpty())
		Expect(result).NotTo(BeNil())
	})

	It("handles nil map", func() {
		var input map[string]string

		result := MapStringsToMapStringAny(input)

		Expect(result).To(BeEmpty())
		Expect(result).NotTo(BeNil())
	})

	It("preserves empty string values", func() {
		input := map[string]string{
			"empty": "",
			"space": " ",
		}

		result := MapStringsToMapStringAny(input)

		Expect(result["empty"]).To(Equal(""))
		Expect(result["space"]).To(Equal(" "))
	})
})

var _ = Describe("ExecutableInPath", func() {
	It("finds executables that exist in PATH", func() {
		// "ls" or "sh" should exist on any Unix system
		path, found, err := ExecutableInPath("sh")
		Expect(found).To(BeTrue())
		Expect(path).NotTo(BeEmpty())
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns false for non-existent executables", func() {
		_, found, err := ExecutableInPath("nonexistent-command-12345")
		Expect(found).To(BeFalse())
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("FileExists", func() {
	It("returns true for existing files", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "exists.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		Expect(err).ToNot(HaveOccurred())

		Expect(FileExists(testFile)).To(BeTrue())
	})

	It("returns true for existing directories", func() {
		tempDir := GinkgoT().TempDir()
		Expect(FileExists(tempDir)).To(BeTrue())
	})

	It("returns false for non-existent paths", func() {
		Expect(FileExists("/nonexistent/path/to/file")).To(BeFalse())
	})
})

var _ = Describe("IsDirectory", func() {
	It("returns true for directories", func() {
		tempDir := GinkgoT().TempDir()
		Expect(IsDirectory(tempDir)).To(BeTrue())
	})

	It("returns false for files", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "file.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		Expect(err).ToNot(HaveOccurred())

		Expect(IsDirectory(testFile)).To(BeFalse())
	})

	It("returns false for non-existent paths", func() {
		Expect(IsDirectory("/nonexistent/path")).To(BeFalse())
	})
})

var _ = Describe("IsJsonObject", func() {
	It("returns true for JSON objects", func() {
		Expect(IsJsonObject([]byte(`{"key": "value"}`))).To(BeTrue())
		Expect(IsJsonObject([]byte(`  {"key": "value"}`))).To(BeTrue())
		Expect(IsJsonObject([]byte(`
		{"key": "value"}`))).To(BeTrue())
	})

	It("returns true for JSON arrays", func() {
		Expect(IsJsonObject([]byte(`[1, 2, 3]`))).To(BeTrue())
		Expect(IsJsonObject([]byte(`  [1, 2, 3]`))).To(BeTrue())
	})

	It("returns false for non-JSON content", func() {
		Expect(IsJsonObject([]byte(`plain text`))).To(BeFalse())
		Expect(IsJsonObject([]byte(`key: value`))).To(BeFalse())
		Expect(IsJsonObject([]byte(`<xml>content</xml>`))).To(BeFalse())
	})

	It("returns false for empty content", func() {
		Expect(IsJsonObject([]byte(``))).To(BeFalse())
		Expect(IsJsonObject([]byte(`   `))).To(BeFalse())
	})
})

var _ = Describe("UntarGz", func() {
	// Helper to create a tar.gz archive in memory
	createTarGz := func(files map[string][]byte, dirs []string) *bytes.Buffer {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		tarWriter := tar.NewWriter(gzWriter)

		for _, dir := range dirs {
			hdr := &tar.Header{
				Name:     dir,
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}
			Expect(tarWriter.WriteHeader(hdr)).To(Succeed())
		}

		for name, content := range files {
			hdr := &tar.Header{
				Name: name,
				Mode: 0644,
				Size: int64(len(content)),
			}
			Expect(tarWriter.WriteHeader(hdr)).To(Succeed())
			_, err := tarWriter.Write(content)
			Expect(err).ToNot(HaveOccurred())
		}

		Expect(tarWriter.Close()).To(Succeed())
		Expect(gzWriter.Close()).To(Succeed())
		return &buf
	}

	It("extracts files from a tar.gz archive", func() {
		tempDir := GinkgoT().TempDir()
		archive := createTarGz(map[string][]byte{
			"file1.txt": []byte("content1"),
			"file2.txt": []byte("content2"),
		}, nil)

		files, err := UntarGz(archive, tempDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(2))

		content1, err := os.ReadFile(filepath.Join(tempDir, "file1.txt"))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(content1)).To(Equal("content1"))

		content2, err := os.ReadFile(filepath.Join(tempDir, "file2.txt"))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(content2)).To(Equal("content2"))
	})

	It("creates directories from archive", func() {
		tempDir := GinkgoT().TempDir()
		archive := createTarGz(nil, []string{"subdir/"})

		_, err := UntarGz(archive, tempDir)
		Expect(err).ToNot(HaveOccurred())

		Expect(filepath.Join(tempDir, "subdir")).To(BeADirectory())
	})

	It("rejects archives with path traversal attempts", func() {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		tarWriter := tar.NewWriter(gzWriter)

		hdr := &tar.Header{
			Name: "../escape.txt",
			Mode: 0644,
			Size: 4,
		}
		Expect(tarWriter.WriteHeader(hdr)).To(Succeed())
		_, err := tarWriter.Write([]byte("evil"))
		Expect(err).ToNot(HaveOccurred())
		Expect(tarWriter.Close()).To(Succeed())
		Expect(gzWriter.Close()).To(Succeed())

		tempDir := GinkgoT().TempDir()
		_, err = UntarGz(&buf, tempDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid tar file"))
	})

	It("rejects archives with unsupported file types", func() {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		tarWriter := tar.NewWriter(gzWriter)

		hdr := &tar.Header{
			Name:     "symlink",
			Mode:     0644,
			Typeflag: tar.TypeSymlink,
			Linkname: "/etc/passwd",
		}
		Expect(tarWriter.WriteHeader(hdr)).To(Succeed())
		Expect(tarWriter.Close()).To(Succeed())
		Expect(gzWriter.Close()).To(Succeed())

		tempDir := GinkgoT().TempDir()
		_, err := UntarGz(&buf, tempDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("only regular files and directories"))
	})

	It("returns error for invalid gzip data", func() {
		tempDir := GinkgoT().TempDir()
		_, err := UntarGz(bytes.NewReader([]byte("not gzip data")), tempDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unzip failed"))
	})
})

var _ = Describe("VersionCmp edge cases", func() {
	It("compares versions with leading zeros lexically", func() {
		// When either has a leading zero, compare as strings
		Expect(VersionCmp("01", "1", false)).To(Equal(-1)) // "01" < "1" lexically
		Expect(VersionCmp("1", "01", false)).To(Equal(1))  // "1" > "01" lexically
	})

	It("compares non-numeric version parts case-insensitively", func() {
		Expect(VersionCmp("1.0.alpha", "1.0.beta", false)).To(Equal(-1))
		Expect(VersionCmp("1.0.ALPHA", "1.0.alpha", false)).To(Equal(0))
	})

	It("handles version a being longer than version b", func() {
		Expect(VersionCmp("1.2.3", "1.2", false)).To(Equal(1))
	})

	It("handles version b being longer than version a", func() {
		Expect(VersionCmp("1.2", "1.2.3", false)).To(Equal(-1))
	})

	It("handles dash vs dot precedence", func() {
		// Dash comes before dot in version comparison
		Expect(VersionCmp("1-0", "1.0", false)).To(Equal(-1))
		Expect(VersionCmp("1.0", "1-0", false)).To(Equal(1))
	})

	It("compares equal versions", func() {
		Expect(VersionCmp("1.2.3", "1.2.3", false)).To(Equal(0))
	})
})

var _ = Describe("DeepMergeMap edge cases", func() {
	It("handles type mismatch when target has map but source has non-map", func() {
		target := map[string]any{
			"key": map[string]any{"nested": "value"},
		}
		source := map[string]any{
			"key": "string value",
		}

		result := DeepMergeMap(target, source)
		Expect(result["key"]).To(Equal("string value"))
	})

	It("handles type mismatch when target has slice but source has non-slice", func() {
		target := map[string]any{
			"key": []any{1, 2, 3},
		}
		source := map[string]any{
			"key": "string value",
		}

		result := DeepMergeMap(target, source)
		Expect(result["key"]).To(Equal("string value"))
	})

	It("adds new keys from source", func() {
		target := map[string]any{
			"existing": "value",
		}
		source := map[string]any{
			"new": "new value",
		}

		result := DeepMergeMap(target, source)
		Expect(result["existing"]).To(Equal("value"))
		Expect(result["new"]).To(Equal("new value"))
	})

	It("handles empty maps", func() {
		target := map[string]any{}
		source := map[string]any{"a": 1}

		result := DeepMergeMap(target, source)
		Expect(result).To(Equal(map[string]any{"a": 1}))

		result2 := DeepMergeMap(source, map[string]any{})
		Expect(result2).To(Equal(map[string]any{"a": 1}))
	})

	It("recursively merges deeply nested maps", func() {
		target := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"key1": "value1",
				},
			},
		}
		source := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"key2": "value2",
				},
			},
		}

		result := DeepMergeMap(target, source)
		level2 := result["level1"].(map[string]any)["level2"].(map[string]any)
		Expect(level2["key1"]).To(Equal("value1"))
		Expect(level2["key2"]).To(Equal("value2"))
	})
})

var _ = Describe("isDigits", func() {
	It("returns true for digit strings", func() {
		Expect(isDigits("123")).To(BeTrue())
		Expect(isDigits("0")).To(BeTrue())
	})

	It("returns false for empty string", func() {
		Expect(isDigits("")).To(BeFalse())
	})

	It("returns false for non-digit strings", func() {
		Expect(isDigits("abc")).To(BeFalse())
		Expect(isDigits("12a3")).To(BeFalse())
	})
})

var _ = Describe("normalize", func() {
	It("removes trailing zeros from version strings", func() {
		Expect(normalize("1.0.0")).To(Equal("1"))
		Expect(normalize("1.2.0")).To(Equal("1.2"))
	})

	It("handles versions with dashes", func() {
		Expect(normalize("1.0-rc1")).To(Equal("1-rc1"))
	})

	It("handles empty string", func() {
		Expect(normalize("")).To(Equal(""))
	})
})
