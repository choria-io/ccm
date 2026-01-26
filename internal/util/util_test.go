// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		Expect(err).ToNot(HaveOccurred())
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

var _ = Describe("FileHasSuffix", func() {
	DescribeTable("suffix matching",
		func(filename string, suffixes []string, expected bool) {
			Expect(FileHasSuffix(filename, suffixes...)).To(Equal(expected))
		},
		// Basic extension matching
		Entry("matches .zip", "archive.zip", []string{".zip", ".tar.gz", ".tgz"}, true),
		Entry("matches .tar.gz", "archive.tar.gz", []string{".zip", ".tar.gz", ".tgz"}, true),
		Entry("matches .tgz", "archive.tgz", []string{".zip", ".tar.gz", ".tgz"}, true),

		// Case insensitive
		Entry("matches .ZIP uppercase", "archive.ZIP", []string{".zip", ".tar.gz", ".tgz"}, true),
		Entry("matches .TAR.GZ uppercase", "archive.TAR.GZ", []string{".zip", ".tar.gz", ".tgz"}, true),
		Entry("matches .TGZ uppercase", "archive.TGZ", []string{".zip", ".tar.gz", ".tgz"}, true),
		Entry("matches mixed case", "archive.Tar.Gz", []string{".zip", ".tar.gz", ".tgz"}, true),

		// Non-matching extensions
		Entry("does not match .tar", "archive.tar", []string{".zip", ".tar.gz", ".tgz"}, false),
		Entry("does not match .gz alone", "archive.gz", []string{".zip", ".tar.gz", ".tgz"}, false),
		Entry("does not match .exe", "archive.exe", []string{".zip", ".tar.gz", ".tgz"}, false),
		Entry("does not match .tar.xz", "archive.tar.xz", []string{".zip", ".tar.gz", ".tgz"}, false),

		// Path handling
		Entry("matches with full path", "/path/to/archive.tar.gz", []string{".zip", ".tar.gz", ".tgz"}, true),
		Entry("matches with complex path", "/var/cache/downloads/v1.0/app.zip", []string{".zip", ".tar.gz", ".tgz"}, true),

		// Edge cases
		Entry("empty filename", "", []string{".zip"}, false),
		Entry("no suffixes provided", "archive.zip", []string{}, false),
		Entry("filename is just extension", ".tar.gz", []string{".tar.gz"}, true),
	)
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

var _ = Describe("LookupUserID", func() {
	It("returns UID for root user", func() {
		uid, err := LookupUserID("root")
		Expect(err).ToNot(HaveOccurred())
		Expect(uid).To(Equal(0))
	})

	It("returns error for non-existent user", func() {
		_, err := LookupUserID("nonexistent_user_12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup user"))
	})

	It("returns error for empty user name", func() {
		_, err := LookupUserID("")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("user name cannot be empty"))
	})
})

var _ = Describe("LookupGroupID", func() {
	It("returns GID for wheel or root group", func() {
		// Try wheel first (macOS), fall back to root (Linux)
		gid, err := LookupGroupID("wheel")
		if err != nil {
			gid, err = LookupGroupID("root")
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(gid).To(Equal(0))
	})

	It("returns error for non-existent group", func() {
		_, err := LookupGroupID("nonexistent_group_12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup group"))
	})

	It("returns error for empty group name", func() {
		_, err := LookupGroupID("")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("group name cannot be empty"))
	})
})

var _ = Describe("LookupOwnerGroup", func() {
	It("returns UID and GID for valid owner and group", func() {
		// Try wheel first (macOS), fall back to root (Linux)
		group := "wheel"
		_, err := LookupGroupID("wheel")
		if err != nil {
			group = "root"
		}

		uid, gid, err := LookupOwnerGroup("root", group)
		Expect(err).ToNot(HaveOccurred())
		Expect(uid).To(Equal(0))
		Expect(gid).To(Equal(0))
	})

	It("returns error for invalid owner", func() {
		_, _, err := LookupOwnerGroup("nonexistent_user_12345", "root")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup user"))
	})

	It("returns error for invalid group", func() {
		_, _, err := LookupOwnerGroup("root", "nonexistent_group_12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup group"))
	})
})

var _ = Describe("ChownFile", func() {
	It("returns error for invalid owner", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		f, err := os.Create(testFile)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		err = ChownFile(f, "nonexistent_user_12345", "root")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup user"))
	})

	It("returns error for invalid group", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		f, err := os.Create(testFile)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		err = ChownFile(f, "root", "nonexistent_group_12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup group"))
	})
})

var _ = Describe("ChownPath", func() {
	It("returns error for invalid owner", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		Expect(err).ToNot(HaveOccurred())

		err = ChownPath(testFile, "nonexistent_user_12345", "root")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup user"))
	})

	It("returns error for invalid group", func() {
		tempDir := GinkgoT().TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		Expect(err).ToNot(HaveOccurred())

		err = ChownPath(testFile, "root", "nonexistent_group_12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("could not lookup group"))
	})

	It("returns error for non-existent path", func() {
		err := ChownPath("/nonexistent/path/to/file", "root", "root")
		Expect(err).To(HaveOccurred())
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

var _ = Describe("CloneMapStrings", func() {
	It("creates a copy of the map", func() {
		source := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result := CloneMapStrings(source)

		Expect(result).To(Equal(source))
		Expect(result).NotTo(BeIdenticalTo(source))
	})

	It("returns an independent copy that can be modified without affecting original", func() {
		source := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result := CloneMapStrings(source)
		result["key1"] = "modified"
		result["key3"] = "new"

		Expect(source["key1"]).To(Equal("value1"))
		Expect(source).NotTo(HaveKey("key3"))
	})

	It("handles empty map", func() {
		source := map[string]string{}

		result := CloneMapStrings(source)

		Expect(result).To(BeEmpty())
		Expect(result).NotTo(BeNil())
	})

	It("handles nil map", func() {
		var source map[string]string

		result := CloneMapStrings(source)

		Expect(result).To(BeEmpty())
		Expect(result).NotTo(BeNil())
	})

	It("preserves empty string values", func() {
		source := map[string]string{
			"empty": "",
			"space": " ",
		}

		result := CloneMapStrings(source)

		Expect(result["empty"]).To(Equal(""))
		Expect(result["space"]).To(Equal(" "))
	})
})

var _ = Describe("RedactUrlCredentials", func() {
	It("returns URL unchanged when no credentials present", func() {
		u, err := url.Parse("https://example.com/path/to/file.yaml")
		Expect(err).ToNot(HaveOccurred())

		result := RedactUrlCredentials(u)
		Expect(result).To(Equal("https://example.com/path/to/file.yaml"))
	})

	It("redacts username and password from URL", func() {
		u, err := url.Parse("https://myuser:secretpass@example.com/path/to/file.yaml")
		Expect(err).ToNot(HaveOccurred())

		result := RedactUrlCredentials(u)
		Expect(result).NotTo(ContainSubstring("myuser"))
		Expect(result).NotTo(ContainSubstring("secretpass"))
		// URL encoding converts [ to %5B and ] to %5D
		Expect(result).To(ContainSubstring("REDACTED"))
		Expect(result).To(ContainSubstring("example.com/path/to/file.yaml"))
	})

	It("redacts username-only credentials from URL", func() {
		u, err := url.Parse("https://myuser@example.com/path/to/file.yaml")
		Expect(err).ToNot(HaveOccurred())

		result := RedactUrlCredentials(u)
		Expect(result).NotTo(ContainSubstring("myuser"))
		// URL encoding converts [ to %5B and ] to %5D
		Expect(result).To(ContainSubstring("REDACTED"))
	})

	It("preserves query parameters and fragments", func() {
		u, err := url.Parse("https://user:pass@example.com/path?query=value#fragment")
		Expect(err).ToNot(HaveOccurred())

		result := RedactUrlCredentials(u)
		Expect(result).To(ContainSubstring("query=value"))
		Expect(result).To(ContainSubstring("fragment"))
	})

	It("preserves port numbers", func() {
		u, err := url.Parse("https://user:pass@example.com:8443/path/to/file.yaml")
		Expect(err).ToNot(HaveOccurred())

		result := RedactUrlCredentials(u)
		Expect(result).To(ContainSubstring(":8443"))
	})
})

var _ = Describe("HttpGet", func() {
	It("returns error for empty URL", func() {
		_, err := HttpGet(context.Background(), "", 0)
		Expect(err).To(MatchError("HTTP request failed: URL is required"))
	})

	It("returns error for invalid URL", func() {
		_, err := HttpGet(context.Background(), "://invalid", 0)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid URL"))
	})

	It("returns error for unsupported scheme", func() {
		_, err := HttpGet(context.Background(), "ftp://example.com/file", 0)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("URL scheme must be http or https"))
	})

	It("returns error when connection fails", func() {
		_, err := HttpGet(context.Background(), "http://localhost:59999/nonexistent", time.Second)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("HTTP request failed"))
	})

	Context("with HTTP test server", func() {
		var server *httptest.Server

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("fetches content from HTTP server", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"key": "value"}`))
			}))

			result, err := HttpGet(context.Background(), server.URL, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.StatusCode).To(Equal(200))
			Expect(string(result.Body)).To(Equal(`{"key": "value"}`))
		})

		It("returns non-200 status codes without error", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("not found"))
			}))

			result, err := HttpGet(context.Background(), server.URL, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.StatusCode).To(Equal(404))
			Expect(result.Status).To(ContainSubstring("404"))
		})

		It("handles Basic Auth from URL credentials", func() {
			var receivedAuth string
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))

			parsedURL, err := url.Parse(server.URL)
			Expect(err).ToNot(HaveOccurred())
			parsedURL.User = url.UserPassword("testuser", "testpass")

			result, err := HttpGet(context.Background(), parsedURL.String(), 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.StatusCode).To(Equal(200))
			Expect(receivedAuth).To(HavePrefix("Basic "))
		})

		It("respects context cancellation", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(5 * time.Second)
				w.WriteHeader(http.StatusOK)
			}))

			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			_, err := HttpGet(ctx, server.URL, time.Minute)
			Expect(err).To(HaveOccurred())
		})

		It("uses default timeout when timeout is zero", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))

			result, err := HttpGet(context.Background(), server.URL, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.StatusCode).To(Equal(200))
		})

		It("uses default timeout when timeout is negative", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))

			result, err := HttpGet(context.Background(), server.URL, -1)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.StatusCode).To(Equal(200))
		})
	})
})

var _ = Describe("FindManifestInFiles", func() {
	It("finds manifest.yaml in file list", func() {
		files := []string{
			"/tmp/extract/config.yaml",
			"/tmp/extract/manifest.yaml",
			"/tmp/extract/data.json",
		}

		result, err := FindManifestInFiles(files, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/tmp/extract/manifest.yaml"))
	})

	It("finds manifest.yaml in nested directory", func() {
		files := []string{
			"/tmp/extract/subdir/manifest.yaml",
			"/tmp/extract/other.yaml",
		}

		result, err := FindManifestInFiles(files, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/tmp/extract/subdir/manifest.yaml"))
	})

	It("strips prefix when provided", func() {
		files := []string{
			"/tmp/extract/manifest.yaml",
		}

		result, err := FindManifestInFiles(files, "/tmp/extract")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/manifest.yaml"))
	})

	It("returns error when manifest.yaml not found", func() {
		files := []string{
			"/tmp/extract/config.yaml",
			"/tmp/extract/data.json",
		}

		_, err := FindManifestInFiles(files, "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("manifest.yaml not found"))
	})

	It("returns error for empty file list", func() {
		files := []string{}

		_, err := FindManifestInFiles(files, "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("manifest.yaml not found"))
	})

	It("does not match files with manifest.yaml in directory name", func() {
		files := []string{
			"/tmp/manifest.yaml.d/config.yaml",
		}

		_, err := FindManifestInFiles(files, "")
		Expect(err).To(HaveOccurred())
	})
})
