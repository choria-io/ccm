// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"

	iu "github.com/choria-io/ccm/internal/util"
)

const (
	ResourceEnsureApiRequestProtocol  = "io.choria.ccm.v1.resource.ensure.request"
	ResourceEnsureApiResponseProtocol = "io.choria.ccm.v1.resource.ensure.response"
)

type RequestEncoding int

var UnknownRequestEncoding RequestEncoding = 0
var JsonRequestEncoding RequestEncoding = 1
var YamlRequestEncoding RequestEncoding = 2

func (e RequestEncoding) String() string {
	switch e {
	case JsonRequestEncoding:
		return "json"
	case YamlRequestEncoding:
		return "yaml"
	default:
		return "unknown"
	}
}

type ResourceEnsureApiRequest struct {
	Protocol   string          `json:"protocol" yaml:"protocol"`
	Type       string          `json:"type" yaml:"type"`
	Properties yaml.RawMessage `json:"properties" yaml:"properties"`
	Encoding   RequestEncoding `json:"-" yaml:"-"`
}

type ResourceEnsureApiResponse struct {
	Protocol string            `json:"protocol" yaml:"protocol"`
	Error    string            `json:"error,omitempty" yaml:"error,omitempty"`
	State    *TransactionEvent `json:"state,omitempty" yaml:"state,omitempty"`
}

func MarshalResourceEnsureApiResponse(encoding RequestEncoding, event *TransactionEvent, err error) ([]byte, error) {
	var r ResourceEnsureApiResponse
	r.Protocol = ResourceEnsureApiResponseProtocol
	if err != nil {
		r.Error = err.Error()
	} else {
		r.State = event
	}

	switch encoding {
	case JsonRequestEncoding:
		return json.MarshalIndent(r, "", "  ")
	case YamlRequestEncoding:
		return yaml.Marshal(r)
	default:
		return nil, fmt.Errorf("unsupported encoding %s", encoding)
	}
}

// UnmarshalResourceEnsureApiRequest unmarshals a resource ensure request, the input request can be YAML or JSON
func UnmarshalResourceEnsureApiRequest(req []byte) (*ResourceEnsureApiRequest, error) {
	var r ResourceEnsureApiRequest
	var err error

	if iu.IsJsonObject(req) {
		err = json.Unmarshal(req, &r)
		r.Encoding = JsonRequestEncoding
	} else {
		err = yaml.Unmarshal(req, &r)
		r.Encoding = YamlRequestEncoding
	}

	if r.Protocol != ResourceEnsureApiRequestProtocol {
		return nil, fmt.Errorf("invalid protocol %q", r.Protocol)
	}

	if r.Type == "" {
		return nil, fmt.Errorf("missing type in request")
	}

	if len(r.Properties) == 0 {
		return nil, fmt.Errorf("missing properties in request")
	}

	return &r, err
}
