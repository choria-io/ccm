// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/choria-io/ccm/internal/metrics"
	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/choria-io/ccm/hiera"
	"github.com/choria-io/ccm/internal/fs"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/templates"
)

// ResourceFactory creates a Resource from properties. Set by the resources
// package at init time to break the import cycle between resources/apply and
// resources.
var ResourceFactory func(ctx context.Context, mgr model.Manager, props model.ResourceProperties) (model.Resource, error)

// Apply represents a parsed and resolved manifest ready for execution
type Apply struct {
	source                 string
	resources              []map[string]model.ResourceProperties
	data                   map[string]any
	failOnError            bool
	overridingHieraData    string
	overridingResolvedData map[string]any
	manifestBytes          []byte
	preMessage             string
	postMessage            string
	currentDepth           int
	maxDepth               int
	skipSession            bool
	denyApplyResources     bool

	mu sync.Mutex
}

const (
	// DefaultMaxRecursionDepth is how many apply() calls deep we can go
	DefaultMaxRecursionDepth = 10
)

func (a *Apply) String() string {
	return fmt.Sprintf("%s with %d resources", a.source, len(a.resources))
}

func (a *Apply) Source() string { return a.source }

func (a *Apply) PreMessage() string  { return a.preMessage }
func (a *Apply) PostMessage() string { return a.postMessage }

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
	if os.Getenv("NO_SCHEMA_VALIDATION") == "1" {
		return nil
	}

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
		source = iu.RedactUrlCredentials(uri)

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
// HTTP Basic Authentication is supported via URL credentials (e.g., https://user:pass@host/path).
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

	// Create a redacted URL for logging (without credentials)
	redactedUrl := iu.RedactUrlCredentials(parsedUrl)

	log.Debug("Getting manifest data from HTTP URL", "url", redactedUrl)

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, manifestUrl, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add Basic Auth if credentials are provided in the URL
	if parsedUrl.User != nil {
		username := parsedUrl.User.Username()
		password, _ := parsedUrl.User.Password()
		req.SetBasicAuth(username, password)
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

	// Store the redacted URL as source to avoid exposing credentials
	apply.(*Apply).source = redactedUrl

	log.Info("Using manifest from HTTP URL in temporary directory", "manifest", manifestPath, "url", redactedUrl)

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

	manifestPath, err := iu.FindManifestInFiles(files, "")
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w in archive", err)
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
	if !filepath.IsAbs(path) {
		wd := mgr.WorkingDirectory()
		if wd != "" {
			path = filepath.Join(wd, path)
		}
	}

	manifestFile, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer manifestFile.Close()

	mgr.SetWorkingDirectory(filepath.Dir(path))

	resolved, apply, err := ResolveManifestReader(ctx, mgr, mgr.WorkingDirectory(), manifestFile, opts...)
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
	PreMessage       string          `json:"pre_message,omitempty" yaml:"pre_message,omitempty"`
	PostMessage      string          `json:"post_message,omitempty" yaml:"post_message,omitempty"`
	ResourcesJetFile string          `json:"resources_jet_file,omitempty" yaml:"resources_jet_file,omitempty"`
	FailOnError      bool            `json:"fail_on_error,omitempty" yaml:"fail_on_error,omitempty"`
	Resources        yaml.RawMessage `json:"resources" yaml:"resources"`
}

// ResolveManifestReader reads and resolves a manifest using Hiera, returning the resolved data and parsed manifest
func ResolveManifestReader(ctx context.Context, mgr model.Manager, dir string, manifest io.Reader, opts ...Option) (map[string]any, model.Apply, error) {
	apply := &Apply{
		source:   "reader",
		maxDepth: DefaultMaxRecursionDepth,
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

	apply.preMessage = parser.CCM.PreMessage
	apply.postMessage = parser.CCM.PostMessage

	result, err := hiera.ResolveYaml(apply.manifestBytes, facts, hiera.DefaultOptions, hieraLogger)
	if err != nil {
		return nil, nil, err
	}

	resolved := result.Data
	rules := result.Rules

	if apply.overridingHieraData != "" {
		overriding, err := hiera.ResolveUrl(ctx, apply.overridingHieraData, mgr, facts, hiera.DefaultOptions, hieraLogger)
		if err != nil {
			return nil, nil, err
		}
		resolved = iu.DeepMergeMap(resolved, overriding.Data)
		rules = append(rules, overriding.Rules...)
	}

	if apply.overridingResolvedData != nil {
		resolved = iu.DeepMergeMap(resolved, apply.overridingResolvedData)
	}

	err = hiera.ValidateData(resolved, rules)
	if err != nil {
		return nil, nil, err
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

	if apply.postMessage != "" {
		parsed, err := templates.ResolveTemplateString(apply.postMessage, env)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid post_message template: %w", err)
		}
		apply.postMessage = parsed
	}
	if apply.preMessage != "" {
		parsed, err := templates.ResolveTemplateString(apply.preMessage, env)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid pre_message template: %w", err)
		}
		apply.preMessage = parsed
	}

	manifestData["resources"] = resources

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

	// Schema validation runs after per-resource parsing so that non-deferred
	// templates have already been resolved into concrete values. For fields
	// that remain templated (e.g. deferred or runtime references) we substitute
	// schema-satisfying placeholders so the structural validator can still
	// inspect required fields, oneOf shapes and additionalProperties without
	// rejecting valid-but-unresolved template expressions.
	validationParser, err := buildValidationParser(parser, apply.resources)
	if err != nil {
		return nil, nil, err
	}

	err = apply.validateManifestAny(validationParser)
	if err != nil {
		return nil, nil, err
	}

	if len(resources) == 0 {
		return nil, nil, fmt.Errorf("manifest must contain resources")
	}

	return manifestData, apply, err
}

// buildValidationParser produces a manifestParser whose resources reflect the
// post-resolution state of the supplied typed resources, with any remaining
// template expressions replaced by their schema_placeholder value. It is used
// solely to feed validateManifestAny and is never returned to callers.
//
// When the manifest produced no resolved resources the original raw resources
// payload is left in place so the schema can flag malformed input (such as a
// null or missing resources key) rather than silently accepting an empty array.
func buildValidationParser(parser manifestParser, resources []map[string]model.ResourceProperties) (manifestParser, error) {
	if len(resources) == 0 {
		return parser, nil
	}

	validationResources := make([]map[string]yaml.RawMessage, 0, len(resources))

	for _, resMap := range resources {
		for typeName, prop := range resMap {
			substituted, err := substituteTemplatesForValidation(prop)
			if err != nil {
				return parser, fmt.Errorf("preparing %s resource for validation: %w", typeName, err)
			}

			propYaml, err := substituted.ToYamlManifest()
			if err != nil {
				return parser, fmt.Errorf("marshaling %s resource for validation: %w", typeName, err)
			}

			validationResources = append(validationResources, map[string]yaml.RawMessage{
				typeName: propYaml,
			})
		}
	}

	rawResources, err := yaml.Marshal(validationResources)
	if err != nil {
		return parser, fmt.Errorf("marshaling resources for validation: %w", err)
	}

	parser.CCM.Resources = rawResources

	return parser, nil
}

func (a *Apply) Execute(ctx context.Context, mgr model.Manager, healthCheckOnly bool, userLog model.Logger) (model.SessionStore, error) {
	if mgr == nil {
		return nil, fmt.Errorf("manager is required")
	}

	if ResourceFactory == nil {
		return nil, fmt.Errorf("ResourceFactory is not initialized; import github.com/choria-io/ccm/resources to register it")
	}

	timer := prometheus.NewTimer(metrics.ManifestApplyTime.WithLabelValues(a.source))
	defer timer.ObserveDuration()

	if mgr.NoopMode() && healthCheckOnly {
		return nil, fmt.Errorf("cannot set healthcheck only and noop mode at the same time")
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

	var session model.SessionStore
	if !a.skipSession {
		session, err = mgr.StartSession(a)
		if err != nil {
			return nil, err
		}
	}

	if a.maxDepthExceeded() {
		return session, fmt.Errorf("maximum apply depth of %d exceeded", a.maxDepth)
	}

	if a.denyApplyResources && a.hasApplyResources() {
		return session, fmt.Errorf("apply resources are denied")
	}

	var terminate bool

	for n, r := range a.Resources() {
		if len(r) > 1 {
			return nil, fmt.Errorf("only one resource type per resource is supported")
		}

		for _, prop := range r {
			if prop == nil {
				userLog.Error("invalid properties received for resource", "resource", n)
				continue
			}

			var event *model.TransactionEvent
			var resource model.Resource
			var err error

			// TODO: error here should rather create a TransactionEvent with an error status
			// TODO: this stuff should be stored in the registry so it knows when to call what so its automatic

			resource, err = ResourceFactory(ctx, mgr, prop)
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

			err = publishRegistration(ctx, mgr, prop, event, log)
			if err != nil {
				return nil, err
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

func (a *Apply) hasApplyResources() bool {
	for _, r := range a.Resources() {
		for _, prop := range r {
			if prop.CommonProperties().Type == "apply" {
				return true
			}
		}
	}

	return false
}

func (a *Apply) maxDepthExceeded() bool {
	if a.maxDepth == 0 {
		a.maxDepth = DefaultMaxRecursionDepth
	}

	return a.currentDepth >= a.maxDepth
}

func publishRegistration(ctx context.Context, mgr model.Manager, prop model.ResourceProperties, event *model.TransactionEvent, log model.Logger) error {
	common := prop.CommonProperties()
	regs := common.RegisterWhenStable
	if len(regs) == 0 {
		return nil
	}
	if mgr.NoopMode() {
		log.Debug("Skipping registration in noop mode", "resource", common.Name)
		return nil
	}
	if event.Failed {
		log.Debug("Skipping registration due to failed event", "resource", common.Name)
		return nil
	}
	if !allHealthChecksPassed(event) {
		log.Debug("Skipping registration due to failed health checks", "resource", common.Name)
		return nil
	}

	for _, reg := range regs {
		log.Debug("Publishing registration", "resource", common.Name, "service", reg.Service)
		err := mgr.PublishRegistration(ctx, reg)
		if err != nil && !errors.Is(err, model.ErrNoRegistrationPublisher) {
			return err
		}
	}

	return nil
}

// allHealthChecksPassed returns true when the event has no health checks or and all checks have OK status
func allHealthChecksPassed(event *model.TransactionEvent) bool {
	if len(event.HealthChecks) == 0 {
		return true
	}

	for _, hc := range event.HealthChecks {
		if hc.Status != model.HealthCheckOK {
			return false
		}
	}

	return true
}
