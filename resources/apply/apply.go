// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/choria-io/ccm/hiera"
	"github.com/choria-io/ccm/internal/fs"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
	execresource "github.com/choria-io/ccm/resources/exec"
	fileresource "github.com/choria-io/ccm/resources/file"
	packageresource "github.com/choria-io/ccm/resources/package"
	serviceresource "github.com/choria-io/ccm/resources/service"
)

// Apply represents a parsed and resolved manifest ready for execution
type Apply struct {
	source                 string
	resources              []map[string]model.ResourceProperties
	data                   map[string]any
	failOnError            bool
	overridingHieraData    string
	overridingResolvedData map[string]any
	manifestBytes          []byte

	mu sync.Mutex
}

func (a *Apply) String() string {
	return fmt.Sprintf("%s with %d resources", a.source, len(a.resources))
}

func (a *Apply) Source() string { return a.source }

func (a *Apply) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(a.toMap())
}

func (a *Apply) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.toMap())
}

func (a *Apply) validateManifestAny(m any) error {
	yp, err := yaml.Marshal(m)
	if err != nil {
		return err
	}

	jp, err := yaml.JSONToYAML(yp)
	if err != nil {
		return err
	}

	return a.validateManifest(jp)
}

func (a *Apply) validateManifest(mb []byte) error {
	if len(mb) == 0 {
		return fmt.Errorf("manifest not parsed")
	}

	jmb, err := yaml.YAMLToJSON(mb)
	if err != nil {
		return err
	}

	rawSchema, err := fs.FS.Open("schemas/manifest.json")
	if err != nil {
		return err
	}
	defer rawSchema.Close()

	parsedSchema, err := jsonschema.UnmarshalJSON(rawSchema)
	if err != nil {
		return fmt.Errorf("invalid manifest schema: %v", err)
	}

	manifestInstance, err := jsonschema.UnmarshalJSON(bytes.NewReader(jmb))
	if err != nil {
		return fmt.Errorf("invalid instance: %v", err)
	}

	c := jsonschema.NewCompiler()
	err = c.AddResource("manifest.json", parsedSchema)
	if err != nil {
		return err
	}

	sch, err := c.Compile("manifest.json")
	if err != nil {
		return err
	}

	return sch.Validate(manifestInstance)
}

func (a *Apply) toMap() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	return map[string]any{
		"ccm": map[string]any{
			"fail_on_error": a.failOnError,
			"resources":     a.resources,
		},
		"data": a.data,
	}
}

// FailOnError returns true if the manifest should fail fast on errors
func (a *Apply) FailOnError() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.failOnError
}

// Resources returns the list of resources in the manifest
func (a *Apply) Resources() []map[string]model.ResourceProperties {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.resources
}

// Data returns the Hiera data associated with the manifest
func (a *Apply) Data() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.data
}

// ResolveManifestUrl parse a url that's either a file or obj://Bucket/Key and resolves the manifest using the correct helpers
//
// When accessing manifests from an object store the working directory of the manager should be set to this directory, it should
// be removed after use and the manager working dir be reset. The wd output from the function is the path to the temporary directory or empty
// string when no temporary directory was used
func ResolveManifestUrl(ctx context.Context, mgr model.Manager, source string, log model.Logger, opts ...Option) (res map[string]any, apply model.Apply, wd string, err error) {
	if source == "" {
		return nil, nil, "", fmt.Errorf("source is required")
	}

	uri, err := url.Parse(source)
	if err != nil {
		return nil, nil, "", err
	}

	switch uri.Scheme {
	case "obj":
		res, apply, wd, err = ResolveManifestObjectValue(ctx, mgr, uri.Host, strings.TrimPrefix(uri.Path, "/"), log, opts...)

	case "http", "https":
		res, apply, wd, err = ResolveManifestHttpUrl(ctx, mgr, source, log, opts...)

	case "":
		res, apply, err = ResolveManifestFilePath(ctx, mgr, source, opts...)

	default:
		return nil, nil, "", fmt.Errorf("unsupported manifest source: %s", source)
	}
	if err != nil {
		return nil, nil, "", err
	}

	apply.(*Apply).source = source

	return res, apply, wd, nil
}

// ResolveManifestHttpUrl reads a manifest from an HTTP(S) URL and resolves it using ResolveManifestReader()
//
// The URL should point to a tar.gz archive containing a manifest.yaml file.
// If a value for `dir` is returned it should be cleaned up after use.
func ResolveManifestHttpUrl(ctx context.Context, mgr model.Manager, manifestUrl string, log model.Logger, opts ...Option) (manifest map[string]any, apply model.Apply, dir string, err error) {
	if manifestUrl == "" {
		return nil, nil, "", fmt.Errorf("URL is required for HTTP manifest source")
	}

	parsedUrl, err := url.Parse(manifestUrl)
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		return nil, nil, "", fmt.Errorf("URL scheme must be http or https, got %q", parsedUrl.Scheme)
	}

	log.Debug("Getting manifest data from HTTP URL", "url", manifestUrl)

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, manifestUrl, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to fetch manifest from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, "", fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	td, err := os.MkdirTemp("", "manifest-*")
	if err != nil {
		return nil, nil, "", err
	}

	resolved, apply, manifestPath, err := unTarAndResolve(ctx, resp.Body, mgr, td, opts...)
	if err != nil {
		os.RemoveAll(td)
		return nil, nil, "", err
	}

	apply.(*Apply).source = manifestUrl

	log.Info("Using manifest from HTTP URL in temporary directory", "manifest", manifestPath, "url", manifestUrl)

	return resolved, apply, filepath.Dir(manifestPath), nil
}

// ResolveManifestObjectValue reads a manifest from an object store and resolves it using ResolveManifestReader()
//
// If a value for `dir` is returned it should be cleaned up after use
func ResolveManifestObjectValue(ctx context.Context, mgr model.Manager, bucket string, file string, log model.Logger, opts ...Option) (manifest map[string]any, apply model.Apply, dir string, err error) {
	if bucket == "" {
		return nil, nil, "", fmt.Errorf("bucket name is required for object store manifest source")
	}
	if file == "" {
		return nil, nil, "", fmt.Errorf("key is required for object store manifest source")
	}

	js, err := mgr.JetStream()
	if err != nil {
		return nil, nil, "", err
	}

	log.Debug("Getting manifest data from JetStream Object Store", "key", file, "bucket", bucket)

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	obj, err := js.ObjectStore(timeoutCtx, bucket)
	if err != nil {
		return nil, nil, "", err
	}

	res, err := obj.Get(timeoutCtx, file)
	if err != nil {
		return nil, nil, "", err
	}
	defer res.Close()

	td, err := os.MkdirTemp("", "manifest-*")
	if err != nil {
		return nil, nil, "", err
	}

	resolved, apply, manifestPath, err := unTarAndResolve(ctx, res, mgr, td, opts...)
	if err != nil {
		os.RemoveAll(td)
		return nil, nil, "", err
	}

	apply.(*Apply).source = fmt.Sprintf("obj://%s/%s", bucket, file)

	log.Info("Using manifest from Object Store in temporary directory", "manifest", manifestPath, "bucket", bucket, "file", file)

	return resolved, apply, filepath.Dir(manifestPath), nil
}

func unTarAndResolve(ctx context.Context, r io.Reader, mgr model.Manager, path string, opts ...Option) (map[string]any, model.Apply, string, error) {
	files, err := iu.UntarGz(r, path)
	if err != nil {
		return nil, nil, "", err
	}

	var manifestPath string
	for _, f := range files {
		if filepath.Base(f) == "manifest.yaml" {
			manifestPath = f
			break
		}
	}

	if manifestPath == "" {
		return nil, nil, "", fmt.Errorf("manifest.yaml not found in object store")
	}
	manifestParent := filepath.Dir(manifestPath)
	mgr.SetWorkingDirectory(manifestParent)

	resolved, apply, err := ResolveManifestFilePath(ctx, mgr, manifestPath, opts...)
	if err != nil {
		return nil, nil, "", err
	}

	return resolved, apply, manifestPath, nil
}

// ResolveManifestFilePath reads a file and resolves the manifest using ResolveManifestReader()
func ResolveManifestFilePath(ctx context.Context, mgr model.Manager, path string, opts ...Option) (map[string]any, model.Apply, error) {
	manifestFile, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer manifestFile.Close()

	resolved, apply, err := ResolveManifestReader(ctx, mgr, filepath.Dir(path), manifestFile, opts...)
	if err != nil {
		return nil, nil, err
	}

	apply.(*Apply).source = path

	return resolved, apply, nil
}

type manifestParser struct {
	Hierarchy yaml.RawMessage         `json:"hierarchy,omitempty" yaml:"hierarchy,omitempty"`
	Data      yaml.RawMessage         `json:"data" yaml:"data"`
	Overrides yaml.RawMessage         `json:"overrides,omitempty" yaml:"overrides,omitempty"`
	CCM       manifestResourcesParser `json:"ccm" yaml:"ccm"`
}

type manifestResourcesParser struct {
	ResourcesJetFile string          `json:"resources_jet_file,omitempty" yaml:"resources_jet_file,omitempty"`
	FailOnError      bool            `json:"fail_on_error,omitempty" yaml:"fail_on_error,omitempty"`
	Resources        yaml.RawMessage `json:"resources" yaml:"resources"`
}

// ResolveManifestReader reads and resolves a manifest using Hiera, returning the resolved data and parsed manifest
func ResolveManifestReader(ctx context.Context, mgr model.Manager, dir string, manifest io.Reader, opts ...Option) (map[string]any, model.Apply, error) {
	apply := &Apply{
		source: "reader",
	}

	for _, o := range opts {
		err := o(apply)
		if err != nil {
			return nil, nil, err
		}
	}

	var err error
	apply.manifestBytes, err = io.ReadAll(manifest)
	if err != nil {
		return nil, nil, err
	}

	facts, err := mgr.Facts(ctx)
	if err != nil {
		return nil, nil, err
	}

	hieraLogger, err := mgr.Logger("hiera", "resources")
	if err != nil {
		return nil, nil, err
	}

	var parser manifestParser
	err = yaml.Unmarshal(apply.manifestBytes, &parser)
	if err != nil {
		return nil, nil, err
	}

	apply.failOnError = parser.CCM.FailOnError

	if parser.CCM.ResourcesJetFile != "" && len(parser.CCM.Resources) > 0 {
		return nil, nil, fmt.Errorf("jet_file and resources cannot be used together")
	}
	if parser.CCM.ResourcesJetFile != "" && dir == "" {
		return nil, nil, fmt.Errorf("jet_file requires a directory to be set")
	}

	resolved, err := hiera.ResolveYaml(apply.manifestBytes, facts, hiera.DefaultOptions, hieraLogger)
	if err != nil {
		return nil, nil, err
	}

	if apply.overridingHieraData != "" {
		overriding, err := hiera.ResolveUrl(ctx, apply.overridingHieraData, mgr, facts, hiera.DefaultOptions, hieraLogger)
		if err != nil {
			return nil, nil, err
		}
		resolved = iu.DeepMergeMap(resolved, overriding)
	}

	if apply.overridingResolvedData != nil {
		resolved = iu.DeepMergeMap(resolved, apply.overridingResolvedData)
	}

	data := mgr.SetData(resolved)
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, nil, err
	}

	env.Data = data
	apply.data = data
	parser.Data, err = yaml.Marshal(data)
	if err != nil {
		return nil, nil, err
	}

	var resources []map[string]yaml.RawMessage
	manifestData := map[string]any{
		"data": data,
	}

	switch {
	case parser.CCM.Resources != nil:
		err = yaml.Unmarshal(parser.CCM.Resources, &resources)
		if err != nil {
			return nil, nil, err
		}

	case parser.CCM.ResourcesJetFile != "":
		path := filepath.Join(dir, parser.CCM.ResourcesJetFile)
		parsed, err := jetParseManifestResources(path, env)
		if err != nil {
			return nil, nil, err
		}
		err = yaml.Unmarshal(parsed, &resources)
		if err != nil {
			return nil, nil, err
		}
		parser.CCM.Resources = parsed
	}

	err = apply.validateManifestAny(parser)
	if err != nil {
		return nil, nil, err
	}

	manifestData["resources"] = resources

	if len(resources) == 0 {
		return nil, nil, fmt.Errorf("manifest must contain resources")
	}

	for i, resource := range resources {
		for typeName, v := range resource {
			props, err := model.NewValidatedResourcePropertiesFromYaml(typeName, v, env)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid manifest resource %d: %w", i+1, err)
			}

			for _, prop := range props {
				apply.resources = append(apply.resources, map[string]model.ResourceProperties{typeName: prop})
			}

		}
	}

	return manifestData, apply, err
}

func (a *Apply) Execute(ctx context.Context, mgr model.Manager, healthCheckOnly bool, userLog model.Logger) (model.SessionStore, error) {
	timer := prometheus.NewTimer(metrics.ManifestApplyTime.WithLabelValues(a.source))
	defer timer.ObserveDuration()

	if mgr.NoopMode() && healthCheckOnly {
		return nil, fmt.Errorf("cannot set healthceck only and noop mode at the same time")
	}

	log, err := mgr.Logger("component", "apply")
	if err != nil {
		return nil, err
	}

	if healthCheckOnly {
		userLog = userLog.With("healthcheck", true)
	}

	if healthCheckOnly {
		userLog.Info("Executing manifest health checks", "manifest", a.Source(), "resources", len(a.Resources()))
	} else {
		userLog.Info("Executing manifest", "manifest", a.Source(), "resources", len(a.Resources()))
	}

	session, err := mgr.StartSession(a)
	if err != nil {
		return nil, err
	}

	var terminate bool

	for _, r := range a.Resources() {
		if len(r) > 1 {
			return nil, fmt.Errorf("only one resource type per resource is supported")
		}

		for _, prop := range r {
			var event *model.TransactionEvent
			var resource model.Resource
			var err error

			// TODO: error here should rather create a TransactionEvent with an error status
			// TODO: this stuff should be stored in the registry so it knows when to call what so its automatic

			switch rprop := prop.(type) {
			case *model.PackageResourceProperties:
				resource, err = packageresource.New(ctx, mgr, *rprop)
			case *model.ServiceResourceProperties:
				resource, err = serviceresource.New(ctx, mgr, *rprop)
			case *model.FileResourceProperties:
				resource, err = fileresource.New(ctx, mgr, *rprop)
			case *model.ExecResourceProperties:
				resource, err = execresource.New(ctx, mgr, *rprop)
			default:
				return nil, fmt.Errorf("unsupported resource property type %T", rprop)
			}
			if err != nil {
				return nil, err
			}

			if healthCheckOnly {
				event, err = resource.Healthcheck(ctx)
			} else {
				event, err = resource.Apply(ctx)
			}
			if err != nil {
				return nil, err
			}

			if event.HealthCheckOnly {
				if len(event.HealthChecks) > 0 {
					event.LogStatus(userLog)
				}
			} else {
				event.LogStatus(userLog)
			}

			err = mgr.RecordEvent(event)
			if err != nil {
				log.Error("Could not save event", "event", event.String())
			}

			if !healthCheckOnly && a.FailOnError() && event.Failed {
				userLog.Warn("Terminating manifest execution due to failed resource")
				terminate = true
			}
		}

		if terminate {
			break
		}
	}

	return session, nil
}
