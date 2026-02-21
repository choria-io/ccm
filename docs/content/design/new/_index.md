+++
title = "Adding a Type"
toc = true
weight = 100
+++

This guide documents the process for adding a new resource type to CCM. It uses the `archive` type as a reference implementation.

## Overview

Adding a new resource type requires changes across several packages:

1. **Model definitions** - Properties, state, and metadata structs
2. **Resource type implementation** - Core logic and provider interface
3. **Provider implementation** - Platform-specific operations
4. **Integration points** - Factory functions and registry
5. **CLI commands** - User-facing command line interface
6. **JSON schemas** - Validation for manifests and API requests
7. **Documentation** - User and design documentation
8. **CCM Studio** - Web-based manifest designer

## File Checklist

| File | Action | Purpose |
|------|--------|---------|
| `model/resource_<type>.go` | Create | Properties, state, metadata structs |
| `model/resource_<type>_test.go` | Create | Property validation tests |
| `model/resource.go` | Modify | Add case to factory function |
| `resources/<type>/<type>.go` | Create | Provider interface definition |
| `resources/<type>/type.go` | Create | Resource type implementation |
| `resources/<type>/type_test.go` | Create | Resource type tests |
| `resources/<type>/provider_mock_test.go` | Generate | Mock provider for tests |
| `resources/<type>/<provider>/factory.go` | Create | Provider factory |
| `resources/<type>/<provider>/<provider>.go` | Create | Provider implementation |
| `resources/<type>/<provider>/<provider>_test.go` | Create | Provider tests |
| `resources/resources.go` | Modify | Add case to `NewResourceFromProperties` |
| `cmd/ensure_<type>.go` | Create | CLI command handler |
| `cmd/ensure.go` | Modify | Register CLI command |
| `internal/fs/schemas/manifest.json` | Modify | Add resource schema definitions |
| `internal/fs/schemas/resource_ensure_request.json` | Modify | Add API request schema |
| `docs/content/resources/<type>.md` | Create | User documentation |
| `docs/content/design/<type>/_index.md` | Create | Design documentation |
| `docs/content/design/<type>/<provider>.md` | Create | Provider documentation |

## Step 1: Model Definitions

Create `model/resource_<type>.go` with the following components.

### Constants

```go
const (
    // ResourceStatus<Type>Protocol is the protocol identifier for <type> resource state
    ResourceStatus<Type>Protocol = "io.choria.ccm.v1.resource.<type>.state"

    // <Type>TypeName is the type name for <type> resources
    <Type>TypeName = "<type>"
)
```

### Properties Struct

The properties struct must satisfy `model.ResourceProperties`:

```go
type ResourceProperties interface {
    CommonProperties() *CommonResourceProperties
    Validate() error
    ResolveTemplates(*templates.Env) error
    ToYamlManifest() (yaml.RawMessage, error)
}
```

Structure:

```go
type <Type>ResourceProperties struct {
    CommonResourceProperties `yaml:",inline"`

    // Add type-specific fields here
    Url      string `json:"url" yaml:"url"`
    Checksum string `json:"checksum,omitempty" yaml:"checksum,omitempty"`
    // ...
}
```

Key points:
- Embed `CommonResourceProperties` with `yaml:",inline"` tag
- Use JSON and YAML struct tags for serialization
- In `Validate()`, call `p.CommonResourceProperties.Validate()` first, then add type-specific validation
- In `ResolveTemplates()`, call `p.CommonResourceProperties.ResolveTemplates(env)` first, then resolve type-specific fields

### State Struct

The state struct must satisfy `model.ResourceState`:

```go
type ResourceState interface {
    CommonState() *CommonResourceState
}
```

Structure:

```go
type <Type>Metadata struct {
    Name     string `json:"name" yaml:"name"`
    Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
    // Add fields describing current system state
}

type <Type>State struct {
    CommonResourceState
    Metadata *<Type>Metadata `json:"metadata,omitempty"`
}
```

### Factory Function

Provide a factory function for YAML parsing:

```go
func New<Type>ResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
    return parseProperties(raw, <Type>TypeName, func() ResourceProperties {
        return &<Type>ResourceProperties{}
    })
}
```

## Step 2: Resource Type Implementation

### Provider Interface (`resources/<type>/<type>.go`)

Define a type-specific provider interface that embeds `model.Provider` and adds type-specific methods:

```go
package <type>resource

import (
    "context"

    "github.com/choria-io/ccm/model"
    "github.com/choria-io/ccm/resources/<type>/<provider>"
)

type <Type>Factory interface {
    model.ProviderFactory
}

func init() {
    <provider>.Register()
}

type <Type>Provider interface {
    model.Provider

    Status(ctx context.Context, properties *model.<Type>ResourceProperties) (*model.<Type>State, error)
    // Add provider-specific methods (e.g., Download, Extract for archive)
}
```

### Type Implementation (`resources/<type>/type.go`)

The Type struct must satisfy both `model.Resource` and `base.EmbeddedResource`:

```go
// model.Resource interface
type Resource interface {
    Type() string
    Name() string
    Provider() string
    Properties() ResourceProperties
    Apply(context.Context) (*TransactionEvent, error)
    Info(context.Context) (any, error)
    Healthcheck(ctx context.Context) (*TransactionEvent, error)
}

// base.EmbeddedResource interface
type EmbeddedResource interface {
    NewTransactionEvent() *model.TransactionEvent
    ApplyResource(ctx context.Context) (model.ResourceState, error)
    SelectProvider() (string, error)
    Type() string
}
```

Embedding `*base.Base` provides implementations for `Apply()`, `Healthcheck()`, `Type()`, `Name()`, `Properties()`, and `NewTransactionEvent()`. The type must implement:
- `ApplyResource()` - core resource application logic
- `SelectProvider()` - provider selection
- `Provider()` - return current provider name
- `Info()` - return resource information

Structure:

```go
type Type struct {
    *base.Base

    prop     *model.<Type>ResourceProperties
    mgr      model.Manager
    log      model.Logger
    provider model.Provider
    facts    map[string]any
    data     map[string]any

    mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)
```

See `resources/archive/type.go` for a complete constructor example.

### ApplyResource Method

The `ApplyResource` method (part of `base.EmbeddedResource`) contains the core logic. It should follow this pattern:

1. Get initial state via `provider.Status()`
2. Check if already in desired state (implement `isDesiredState()` helper)
3. If stable, call `t.FinalizeState()` and return early
4. Apply changes, respecting `t.mgr.NoopMode()`
5. Get final state and verify desired state was achieved
6. Call `t.FinalizeState()` with appropriate flags

See `resources/archive/type.go:ApplyResource()` for a complete example.

### Provider Selection Methods

The `SelectProvider()` method should use `registry.FindSuitableProvider()` to select an appropriate provider. See `resources/archive/type.go` for the standard implementation pattern.

## Step 3: Provider Implementation

### Factory (`resources/<type>/<provider>/factory.go`)

The factory must satisfy `model.ProviderFactory`:

```go
type ProviderFactory interface {
    TypeName() string
    Name() string
    New(log Logger, runner CommandRunner) (Provider, error)
    IsManageable(facts map[string]any, properties ResourceProperties) (bool, int, error)
}
```

The `IsManageable` method returns:
- `bool` - whether this provider can manage the resource
- `int` - priority (higher wins when multiple providers match)
- `error` - any error encountered

Structure:

```go
package <provider>

import (
    "github.com/choria-io/ccm/internal/registry"
    "github.com/choria-io/ccm/model"
)

const ProviderName = "<provider>"

func Register() {
    registry.MustRegister(&factory{})
}

type factory struct{}

func (p *factory) TypeName() string { return model.<Type>TypeName }
func (p *factory) Name() string     { return ProviderName }
func (p *factory) New(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
    return New<Provider>Provider(log, runner)
}
func (p *factory) IsManageable(facts map[string]any, prop model.ResourceProperties) (bool, int, error) {
    // Type assert and check if this provider can handle the resource
    return true, 1, nil
}
```

See `resources/archive/http/factory.go` for a complete example.

### Provider Implementation (`resources/<type>/<provider>/<provider>.go`)

The provider must satisfy the type-specific provider interface defined in Step 2 (which embeds `model.Provider`):

```go
type Provider interface {
    Name() string
}
```

Structure:

```go
package <provider>

import (
    "context"

    "github.com/choria-io/ccm/model"
)

type <Provider>Provider struct {
    log    model.Logger
    runner model.CommandRunner
}

func New<Provider>Provider(log model.Logger, runner model.CommandRunner) (*<Provider>Provider, error) {
    return &<Provider>Provider{log: log, runner: runner}, nil
}

func (p *<Provider>Provider) Name() string {
    return ProviderName
}

func (p *<Provider>Provider) Status(ctx context.Context, properties *model.<Type>ResourceProperties) (*model.<Type>State, error) {
    state := &model.<Type>State{
        CommonResourceState: model.NewCommonResourceState(
            model.ResourceStatus<Type>Protocol,
            model.<Type>TypeName,
            properties.Name,
            model.EnsureAbsent,
        ),
        Metadata: &model.<Type>Metadata{
            Name:     properties.Name,
            Provider: ProviderName,
        },
    }

    // Query system state and populate metadata

    return state, nil
}

// Implement other type-specific provider methods...
```

See `resources/archive/http/http.go` for a complete example.

## Step 4: Integration Points

### Update `resources/resources.go`

Add the import and case statement:

```go
import (
    // ...
    <type>resource "github.com/choria-io/ccm/resources/<type>"
)

func NewResourceFromProperties(ctx context.Context, mgr model.Manager, props model.ResourceProperties) (model.Resource, error) {
    switch rprop := props.(type) {
    // ... existing cases ...
    case *model.<Type>ResourceProperties:
        return <type>resource.New(ctx, mgr, *rprop)
    default:
        return nil, fmt.Errorf("unsupported resource property type %T", rprop)
    }
}
```

### Update `model/resource.go`

Add the case to `NewResourcePropertiesFromYaml`:

```go
func NewResourcePropertiesFromYaml(typeName string, rawProperties yaml.RawMessage, env *templates.Env) ([]ResourceProperties, error) {
    switch typeName {
    // ... existing cases ...
    case <Type>TypeName:
        props, err = New<Type>ResourcePropertiesFromYaml(rawProperties)
    default:
        return nil, fmt.Errorf("%w: %s %s", ErrResourceInvalid, ErrUnknownType, typeName)
    }
    // ...
}
```

## Step 5: CLI Command

Create `cmd/ensure_<type>.go`:

```go
package main

import (
    "github.com/choria-io/ccm/model"
    "github.com/choria-io/fisk"
)

type ensure<Type>Command struct {
    name string
    // Add command-specific fields for flags

    parent *ensureCommand
}

func registerEnsure<Type>Command(ccm *fisk.CmdClause, parent *ensureCommand) {
    cmd := &ensure<Type>Command{parent: parent}

    <type> := ccm.Command("<type>", "<Type> management").Action(cmd.<type>Action)
    <type>.Arg("name", "Resource name").Required().StringVar(&cmd.name)
    // Add type-specific flags

    parent.addCommonFlags(<type>)
}

func (c *ensure<Type>Command) <type>Action(_ *fisk.ParseContext) error {
    properties := model.<Type>ResourceProperties{
        CommonResourceProperties: model.CommonResourceProperties{
            Name:     c.name,
            Ensure:   model.EnsurePresent,
            Provider: c.parent.provider,
        },
        // Set type-specific properties from flags
    }

    return c.parent.commonEnsureResource(&properties)
}
```

Update `cmd/ensure.go`:

```go
func registerEnsureCommand(ccm *fisk.Application) {
    // ... existing code ...
    registerEnsure<Type>Command(ens, cmd)
}
```

## Step 6: JSON Schemas

### Update `internal/fs/schemas/manifest.json`

Add to the `$defs/resource` properties:

```json
"<type>": {
  "oneOf": [
    { "$ref": "#/$defs/<type>ResourceList" },
    { "$ref": "#/$defs/<type>ResourcePropertiesWithName" }
  ]
}
```

Add resource list definition:

```json
"<type>ResourceList": {
  "type": "array",
  "description": "List of <type> resources to manage (named format)",
  "items": {
    "type": "object",
    "additionalProperties": {
      "$ref": "#/$defs/<type>ResourceProperties"
    },
    "minProperties": 1,
    "maxProperties": 1
  }
}
```

Add properties definitions:

```json
"<type>ResourcePropertiesWithName": {
  "allOf": [
    { "$ref": "#/$defs/<type>ResourceProperties" },
    {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "description": "Resource name"
        }
      },
      "required": ["name"]
    }
  ]
},
"<type>ResourceProperties": {
  "type": "object",
  "properties": {
    "ensure": {
      "type": "string",
      "enum": ["present", "absent"]
    }
    // Add type-specific properties
  }
}
```

### Update `internal/fs/schemas/resource_ensure_request.json`

Add to the `type` enum:

```json
"enum": ["package", "service", "file", "exec", "archive", "<type>"]
```

Add to `properties.oneOf`:

```json
{ "$ref": "#/$defs/<type>Properties" }
```

Add properties definition under `$defs`:

```json
"<type>Properties": {
  "allOf": [
    { "$ref": "#/$defs/commonProperties" },
    {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "description": "Resource name"
        }
        // Add type-specific properties
      },
      "required": ["name"]
    }
  ]
}
```

## Step 7: Generate Mocks

Generate the provider mock for tests:

```bash
mockgen -write_generate_directive \
  -source resources/<type>/<type>.go \
  -destination resources/<type>/provider_mock_test.go \
  -package <type>resource
```

Or use the project command:

```bash
abt gen mocks
```

## Step 8: Testing

### Model Tests (`model/resource_<type>_test.go`)

Test property validation:

```go
var _ = Describe("<Type>ResourceProperties", func() {
    Describe("Validate", func() {
        It("should require name", func() {
            p := &model.<Type>ResourceProperties{}
            p.Ensure = model.EnsurePresent
            Expect(p.Validate()).To(MatchError(model.ErrResourceNameRequired))
        })

        It("should validate ensure values", func() {
            p := &model.<Type>ResourceProperties{}
            p.Name = "test"
            p.Ensure = "invalid"
            Expect(p.Validate()).To(HaveOccurred())
        })
    })
})
```

### Type Tests (`resources/<type>/type_test.go`)

Use the mock manager helper:

```go
var _ = Describe("<Type> Type", func() {
    var mockctl *gomock.Controller

    BeforeEach(func() {
        mockctl = gomock.NewController(GinkgoT())
        registry.Clear()
        // Register mock factory
    })

    AfterEach(func() {
        mockctl.Finish()
    })

    Describe("Apply", func() {
        It("should handle present ensure state", func() {
            mgr, _ := modelmocks.NewManager(facts, data, false, mockctl)
            // Test implementation
        })
    })
})
```

## Key Patterns

### State Checking

Always check current state before making changes:

```go
initialStatus, err := p.Status(ctx, t.prop)
if err != nil {
    return nil, err
}

isStable := t.isDesiredState(properties, initialStatus)
if isStable {
    // No changes needed
    t.FinalizeState(initialStatus, noop, "", false, true, false)
    return initialStatus, nil
}
```

### Noop Mode

All resources must respect noop mode:

```go
if !noop {
    // Make actual changes
    t.log.Info("Applying changes")
    err = p.SomeAction(ctx, properties)
} else {
    t.log.Info("Skipping changes as noop")
    noopMessage = "Would have applied changes"
}
```

### Error Handling

Use sentinel errors from `model/errors.go`:

```go
var (
    ErrResourceInvalid    = errors.New("resource invalid")
    ErrProviderNotFound   = errors.New("provider not found")
    ErrNoSuitableProvider = errors.New("no suitable provider")
    ErrDesiredStateFailed = errors.New("desired state not achieved")
)
```

Wrap errors with context:

```go
err := os.Remove(path)
if err != nil {
    return fmt.Errorf("could not remove file: %w", err)
}
```

### Template Resolution

The `ResolveTemplates` method (part of `model.ResourceProperties`) should resolve all user-facing string fields using `templates.ResolveTemplateString()`. Always call the embedded `CommonResourceProperties.ResolveTemplates(env)` first.

### Provider Selection

Providers declare manageability via `IsManageable` on the factory (see `model.ProviderFactory` in Step 3). Multiple providers can match; the one with highest priority is selected.

## Documentation

Create user documentation in `docs/content/resources/<type>.md` covering:
- Overview and use cases
- Ensure states table
- Properties table with descriptions
- Usage examples (manifest, CLI, API)

Create design documentation in `docs/content/design/<type>/_index.md` covering:
- Provider interface specification
- State checking logic
- Apply logic flowchart

Create provider documentation in `docs/content/design/<type>/<provider>.md` covering:
- Provider selection criteria
- Platform requirements
- Implementation details

## CCM Studio

CCM Studio is a web-based manifest designer. After adding a new resource type, update CCM Studio to support it:

> [!info] Note
> CCM Studio is a closed-source project. The maintainers will complete this step.

- Add the new resource type to the resource palette
- Create property editors for type-specific fields
- Add validation matching the JSON schema definitions
- Update any resource type documentation or help text