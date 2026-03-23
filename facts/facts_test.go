// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	"go.uber.org/mock/gomock"
)

func TestFacts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Facts")
}

var _ = Describe("Facts", func() {
	var (
		ctx     context.Context
		logger  *modelmocks.MockLogger
		mockctl *gomock.Controller
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("Gather", func() {
		It("should return standard facts", func() {
			opts := model.FactsConfig{
				NoCPUFacts:       true,
				NoMemoryFacts:    true,
				NoPartitionFacts: true,
				NoHostFacts:      true,
				NoNetworkFacts:   true,
			}

			result, err := Gather(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKey("host"))
			Expect(result).To(HaveKey("network"))
			Expect(result).To(HaveKey("partition"))
			Expect(result).To(HaveKey("cpu"))
			Expect(result).To(HaveKey("memory"))
		})

		It("should merge file facts", func() {
			td := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(td, "facts.json"), []byte(`{"custom":"value"}`), 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
				NoCPUFacts:            true,
				NoMemoryFacts:         true,
				NoPartitionFacts:      true,
				NoHostFacts:           true,
				NoNetworkFacts:        true,
			}

			result, err := Gather(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("custom", "value"))
		})

		It("should include extra fact sources", func() {
			extra := func(_ context.Context, _ model.FactsConfig, _ model.Logger) (map[string]any, error) {
				return map[string]any{"extra": "data"}, nil
			}

			opts := model.FactsConfig{
				NoCPUFacts:       true,
				NoMemoryFacts:    true,
				NoPartitionFacts: true,
				NoHostFacts:      true,
				NoNetworkFacts:   true,
				ExtraFactSources: []model.FactProvider{extra},
			}

			result, err := Gather(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("extra", "data"))
		})

		It("should continue when an extra fact source fails", func() {
			failing := func(_ context.Context, _ model.FactsConfig, _ model.Logger) (map[string]any, error) {
				return nil, os.ErrNotExist
			}
			working := func(_ context.Context, _ model.FactsConfig, _ model.Logger) (map[string]any, error) {
				return map[string]any{"working": true}, nil
			}

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			opts := model.FactsConfig{
				NoCPUFacts:       true,
				NoMemoryFacts:    true,
				NoPartitionFacts: true,
				NoHostFacts:      true,
				NoNetworkFacts:   true,
				ExtraFactSources: []model.FactProvider{failing, working},
			}

			result, err := Gather(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("working", true))
		})
	})

	Describe("gatherFileFacts", func() {
		It("should skip empty directories", func() {
			opts := model.FactsConfig{}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should read JSON facts files", func() {
			td := GinkgoT().TempDir()
			factsData := map[string]any{"role": "webserver", "env": "production"}
			jb, _ := json.Marshal(factsData)
			Expect(os.WriteFile(filepath.Join(td, "facts.json"), jb, 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("role", "webserver"))
			Expect(result).To(HaveKeyWithValue("env", "production"))
		})

		It("should read YAML facts files", func() {
			td := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(td, "facts.yaml"), []byte("role: database\nenv: staging\n"), 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("role", "database"))
			Expect(result).To(HaveKeyWithValue("env", "staging"))
		})

		It("should merge JSON and YAML facts with YAML taking precedence", func() {
			td := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(td, "facts.json"), []byte(`{"role":"webserver","port":8080}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(td, "facts.yaml"), []byte("role: database\nregion: us-east\n"), 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			// YAML is read after JSON, so YAML values win
			Expect(result).To(HaveKeyWithValue("role", "database"))
			Expect(result).To(HaveKeyWithValue("region", "us-east"))
		})

		It("should merge facts from system and user directories", func() {
			sysDir := GinkgoT().TempDir()
			userDir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(sysDir, "facts.json"), []byte(`{"source":"system"}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(userDir, "facts.json"), []byte(`{"source":"user","extra":"data"}`), 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: sysDir,
				UserConfigDirectory:   userDir,
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			// User dir is read after system dir, so user values win
			Expect(result).To(HaveKeyWithValue("source", "user"))
			Expect(result).To(HaveKeyWithValue("extra", "data"))
		})

		It("should handle invalid JSON gracefully", func() {
			td := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(td, "facts.json"), []byte(`{invalid json`), 0644)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should handle invalid YAML gracefully", func() {
			td := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(td, "facts.yaml"), []byte(":\n  :\n    - [invalid"), 0644)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
			}

			_, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject relative paths", func() {
			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			opts := model.FactsConfig{
				SystemConfigDirectory: "relative/path",
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should clean paths with traversal components", func() {
			td := GinkgoT().TempDir()
			sub := filepath.Join(td, "sub")
			Expect(os.Mkdir(sub, 0755)).To(Succeed())
			// Write facts into td, reference it via td/sub/..
			Expect(os.WriteFile(filepath.Join(td, "facts.json"), []byte(`{"cleaned":"yes"}`), 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: filepath.Join(sub, ".."),
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("cleaned", "yes"))
		})
	})

	Describe("readFactsFile", func() {
		It("should return nil for non-existent file", func() {
			result := readFactsFile("/nonexistent/facts.json", json.Unmarshal, logger)
			Expect(result).To(BeNil())
		})

		It("should parse valid JSON", func() {
			td := GinkgoT().TempDir()
			path := filepath.Join(td, "facts.json")
			Expect(os.WriteFile(path, []byte(`{"key":"value","num":42}`), 0644)).To(Succeed())

			result := readFactsFile(path, json.Unmarshal, logger)
			Expect(result).To(HaveKeyWithValue("key", "value"))
			Expect(result).To(HaveKeyWithValue("num", BeNumerically("==", 42)))
		})

		It("should parse valid YAML", func() {
			td := GinkgoT().TempDir()
			path := filepath.Join(td, "facts.yaml")
			Expect(os.WriteFile(path, []byte("key: value\nnum: 42\n"), 0644)).To(Succeed())

			result := readFactsFile(path, yaml.Unmarshal, logger)
			Expect(result).To(HaveKeyWithValue("key", "value"))
			Expect(result).To(HaveKeyWithValue("num", BeNumerically("==", 42)))
		})

		It("should return nil for invalid content", func() {
			td := GinkgoT().TempDir()
			path := filepath.Join(td, "facts.json")
			Expect(os.WriteFile(path, []byte(`not json`), 0644)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			result := readFactsFile(path, json.Unmarshal, logger)
			Expect(result).To(BeNil())
		})

		It("should return nil for unreadable file", func() {
			td := GinkgoT().TempDir()
			path := filepath.Join(td, "facts.json")
			Expect(os.WriteFile(path, []byte(`{}`), 0000)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			result := readFactsFile(path, json.Unmarshal, logger)
			Expect(result).To(BeNil())
		})

		It("should skip symlinked files", func() {
			td := GinkgoT().TempDir()
			real := filepath.Join(td, "real.json")
			link := filepath.Join(td, "facts.json")
			Expect(os.WriteFile(real, []byte(`{"key":"value"}`), 0644)).To(Succeed())
			Expect(os.Symlink(real, link)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			result := readFactsFile(link, json.Unmarshal, logger)
			Expect(result).To(BeNil())
		})
	})

	Describe("readFactsDir", func() {
		It("should return nil for non-existent directory", func() {
			result := readFactsDir("/nonexistent/facts.d", logger)
			Expect(result).To(BeNil())
		})

		It("should return empty map for empty directory", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())

			result := readFactsDir(dir, logger)
			Expect(result).To(BeEmpty())
		})

		It("should read JSON and YAML files", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "net.json"), []byte(`{"network":"lan"}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "app.yaml"), []byte("app: myapp\n"), 0644)).To(Succeed())

			result := readFactsDir(dir, logger)
			Expect(result).To(HaveKeyWithValue("app", "myapp"))
			Expect(result).To(HaveKeyWithValue("network", "lan"))
		})

		It("should skip .yml files", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"key":"value"}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "extra.yml"), []byte("extra: yes\n"), 0644)).To(Succeed())

			result := readFactsDir(dir, logger)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("key", "value"))
		})

		It("should process files in sorted filename order", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "2override.json"), []byte(`{"role":"override"}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "1base.json"), []byte(`{"role":"base","source":"base"}`), 0644)).To(Succeed())

			result := readFactsDir(dir, logger)
			// 1base.json < 2override.json, so 2override wins
			Expect(result).To(HaveKeyWithValue("role", "override"))
			Expect(result).To(HaveKeyWithValue("source", "base"))
		})

		It("should skip non-json/yaml files", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"key":"value"}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not facts"), 0644)).To(Succeed())

			result := readFactsDir(dir, logger)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("key", "value"))
		})

		It("should skip subdirectories", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(dir, "subdir"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"key":"value"}`), 0644)).To(Succeed())

			result := readFactsDir(dir, logger)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("key", "value"))
		})

		It("should skip symlinked files in directory", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())

			real := filepath.Join(td, "real.json")
			Expect(os.WriteFile(real, []byte(`{"key":"value"}`), 0644)).To(Succeed())
			Expect(os.Symlink(real, filepath.Join(dir, "link.json"))).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "regular.json"), []byte(`{"regular":"yes"}`), 0644)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			result := readFactsDir(dir, logger)
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("regular", "yes"))
		})

		It("should skip symlinked directory", func() {
			td := GinkgoT().TempDir()
			realDir := filepath.Join(td, "real.d")
			Expect(os.Mkdir(realDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(realDir, "data.json"), []byte(`{"key":"value"}`), 0644)).To(Succeed())

			linkDir := filepath.Join(td, "facts.d")
			Expect(os.Symlink(realDir, linkDir)).To(Succeed())

			logger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			result := readFactsDir(linkDir, logger)
			Expect(result).To(BeNil())
		})

		It("should be merged via gatherFileFacts", func() {
			td := GinkgoT().TempDir()
			dir := filepath.Join(td, "facts.d")
			Expect(os.Mkdir(dir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(td, "facts.json"), []byte(`{"base":"value"}`), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "extra.json"), []byte(`{"extra":"data"}`), 0644)).To(Succeed())

			opts := model.FactsConfig{
				SystemConfigDirectory: td,
			}

			result, err := gatherFileFacts(ctx, opts, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("base", "value"))
			Expect(result).To(HaveKeyWithValue("extra", "data"))
		})
	})

	Describe("getMemoryFacts", func() {
		It("should return empty facts when NoMemoryFacts is set", func() {
			opts := &model.FactsConfig{NoMemoryFacts: true}
			result := getMemoryFacts(ctx, opts)

			Expect(result).To(HaveKey("swap"))
			Expect(result).To(HaveKey("virtual"))
			Expect(result["virtual"]).To(Equal(map[string]any{}))
		})

		It("should skip swap when NoSwapFacts is set", func() {
			opts := &model.FactsConfig{NoSwapFacts: true}
			result := getMemoryFacts(ctx, opts)

			swap, ok := result["swap"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(swap["info"]).To(Equal(map[string]any{}))
			Expect(swap["devices"]).To(Equal(map[string]any{}))
		})

		It("should gather memory facts when enabled", func() {
			opts := &model.FactsConfig{}
			result := getMemoryFacts(ctx, opts)

			Expect(result).To(HaveKey("virtual"))
			Expect(result).To(HaveKey("swap"))
		})
	})

	Describe("getCpuFacts", func() {
		It("should return empty facts when NoCPUFacts is set", func() {
			opts := &model.FactsConfig{NoCPUFacts: true}
			result := getCpuFacts(ctx, opts)

			Expect(result["info"]).To(Equal([]any{}))
		})

		It("should gather CPU facts when enabled", func() {
			opts := &model.FactsConfig{}
			result := getCpuFacts(ctx, opts)

			Expect(result).To(HaveKey("info"))
		})
	})

	Describe("getPartitionFacts", func() {
		It("should return empty facts when NoPartitionFacts is set", func() {
			opts := &model.FactsConfig{NoPartitionFacts: true}
			result := getPartitionFacts(ctx, opts)

			Expect(result["partitions"]).To(Equal([]any{}))
			Expect(result["usage"]).To(Equal([]any{}))
		})

		It("should gather partition facts when enabled", func() {
			opts := &model.FactsConfig{}
			result := getPartitionFacts(ctx, opts)

			Expect(result).To(HaveKey("partitions"))
			Expect(result).To(HaveKey("usage"))
		})
	})

	Describe("getHostFacts", func() {
		It("should return empty facts when NoHostFacts is set", func() {
			opts := &model.FactsConfig{NoHostFacts: true}
			result := getHostFacts(ctx, opts)

			Expect(result["info"]).To(Equal(map[string]any{}))
		})

		It("should gather host facts when enabled", func() {
			opts := &model.FactsConfig{}
			result := getHostFacts(ctx, opts)

			Expect(result).To(HaveKey("info"))
		})
	})

	Describe("getNetworkFacts", func() {
		It("should return empty facts when NoNetworkFacts is set", func() {
			opts := &model.FactsConfig{NoNetworkFacts: true}
			result := getNetworkFacts(ctx, opts)

			Expect(result["addresses"]).To(Equal([]any{}))
			Expect(result["interfaces"]).To(Equal([]any{}))
			Expect(result["default_ipv4"]).To(Equal(""))
			Expect(result["default_ipv6"]).To(Equal(""))
		})

		It("should gather network facts when enabled", func() {
			opts := &model.FactsConfig{}
			result := getNetworkFacts(ctx, opts)

			Expect(result).To(HaveKey("interfaces"))
			Expect(result).To(HaveKey("addresses"))
			Expect(result).To(HaveKey("default_ipv4"))
			Expect(result).To(HaveKey("default_ipv6"))
		})
	})
})
