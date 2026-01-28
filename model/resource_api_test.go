// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"errors"

	"github.com/goccy/go-yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceEnsureApi", func() {
	Describe("Constants", func() {
		It("Should have correct protocol values", func() {
			Expect(ResourceEnsureApiRequestProtocol).To(Equal("io.choria.ccm.v1.resource.ensure.request"))
			Expect(ResourceEnsureApiResponseProtocol).To(Equal("io.choria.ccm.v1.resource.ensure.response"))
		})
	})

	Describe("RequestEncoding", func() {
		Describe("String", func() {
			DescribeTable("encoding string conversion",
				func(encoding RequestEncoding, expected string) {
					Expect(encoding.String()).To(Equal(expected))
				},
				Entry("json encoding", JsonRequestEncoding, "json"),
				Entry("yaml encoding", YamlRequestEncoding, "yaml"),
				Entry("unknown encoding", UnknownRequestEncoding, "unknown"),
				Entry("invalid encoding value", RequestEncoding(99), "unknown"),
			)
		})
	})

	Describe("UnmarshalResourceEnsureApiRequest", func() {
		It("Should unmarshal valid JSON request", func() {
			jsonReq := `{
				"protocol": "io.choria.ccm.v1.resource.ensure.request",
				"type": "package",
				"properties": {"name": "nginx", "ensure": "present"}
			}`

			req, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).ToNot(HaveOccurred())
			Expect(req).ToNot(BeNil())
			Expect(req.Protocol).To(Equal(ResourceEnsureApiRequestProtocol))
			Expect(req.Type).To(Equal("package"))
			Expect(req.Encoding).To(Equal(JsonRequestEncoding))
			Expect(req.Properties).ToNot(BeEmpty())
		})

		It("Should unmarshal valid YAML request", func() {
			yamlReq := `
protocol: io.choria.ccm.v1.resource.ensure.request
type: service
properties:
  name: nginx
  ensure: running
`
			req, err := UnmarshalResourceEnsureApiRequest([]byte(yamlReq))
			Expect(err).ToNot(HaveOccurred())
			Expect(req).ToNot(BeNil())
			Expect(req.Protocol).To(Equal(ResourceEnsureApiRequestProtocol))
			Expect(req.Type).To(Equal("service"))
			Expect(req.Encoding).To(Equal(YamlRequestEncoding))
			Expect(req.Properties).ToNot(BeEmpty())
		})

		It("Should reject request with invalid protocol", func() {
			jsonReq := `{
				"protocol": "invalid.protocol",
				"type": "package",
				"properties": {"name": "nginx"}
			}`

			_, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid protocol"))
		})

		It("Should reject request with missing type", func() {
			jsonReq := `{
				"protocol": "io.choria.ccm.v1.resource.ensure.request",
				"properties": {"name": "nginx"}
			}`

			_, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing type"))
		})

		It("Should reject request with missing properties", func() {
			jsonReq := `{
				"protocol": "io.choria.ccm.v1.resource.ensure.request",
				"type": "package"
			}`

			_, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing properties"))
		})

		It("Should accept request with empty object properties", func() {
			// Note: {} serializes to a non-empty byte slice, so this is valid
			jsonReq := `{
				"protocol": "io.choria.ccm.v1.resource.ensure.request",
				"type": "package",
				"properties": {}
			}`

			req, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).ToNot(HaveOccurred())
			Expect(req.Type).To(Equal("package"))
		})

		It("Should handle JSON array detection", func() {
			// Arrays should be detected as JSON
			jsonReq := `[{"protocol": "io.choria.ccm.v1.resource.ensure.request"}]`

			_, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).To(HaveOccurred())
		})

		It("Should handle whitespace in JSON", func() {
			jsonReq := `  {
				"protocol": "io.choria.ccm.v1.resource.ensure.request",
				"type": "file",
				"properties": {"name": "/etc/test"}
			}`

			req, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).ToNot(HaveOccurred())
			Expect(req.Encoding).To(Equal(JsonRequestEncoding))
		})
	})

	Describe("MarshalResourceEnsureApiResponse", func() {
		It("Should marshal successful response as JSON", func() {
			event := NewTransactionEvent("package", "nginx", "")
			event.Changed = true
			event.RequestedEnsure = "present"

			data, err := MarshalResourceEnsureApiResponse(JsonRequestEncoding, event, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeEmpty())

			var resp ResourceEnsureApiResponse
			err = json.Unmarshal(data, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Protocol).To(Equal(ResourceEnsureApiResponseProtocol))
			Expect(resp.Error).To(BeEmpty())
			Expect(resp.State).ToNot(BeNil())
			Expect(resp.State.Name).To(Equal("nginx"))
			Expect(resp.State.Changed).To(BeTrue())
		})

		It("Should marshal successful response as YAML", func() {
			event := NewTransactionEvent("service", "httpd", "webserver")
			event.Changed = false
			event.RequestedEnsure = "running"

			data, err := MarshalResourceEnsureApiResponse(YamlRequestEncoding, event, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeEmpty())

			var resp ResourceEnsureApiResponse
			err = yaml.Unmarshal(data, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Protocol).To(Equal(ResourceEnsureApiResponseProtocol))
			Expect(resp.Error).To(BeEmpty())
			Expect(resp.State).ToNot(BeNil())
			Expect(resp.State.Name).To(Equal("httpd"))
			Expect(resp.State.Alias).To(Equal("webserver"))
		})

		It("Should marshal error response as JSON", func() {
			testErr := errors.New("resource not found")

			data, err := MarshalResourceEnsureApiResponse(JsonRequestEncoding, nil, testErr)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeEmpty())

			var resp ResourceEnsureApiResponse
			err = json.Unmarshal(data, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Protocol).To(Equal(ResourceEnsureApiResponseProtocol))
			Expect(resp.Error).To(Equal("resource not found"))
			Expect(resp.State).To(BeNil())
		})

		It("Should marshal error response as YAML", func() {
			testErr := errors.New("provider not available")

			data, err := MarshalResourceEnsureApiResponse(YamlRequestEncoding, nil, testErr)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).ToNot(BeEmpty())

			var resp ResourceEnsureApiResponse
			err = yaml.Unmarshal(data, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Protocol).To(Equal(ResourceEnsureApiResponseProtocol))
			Expect(resp.Error).To(Equal("provider not available"))
			Expect(resp.State).To(BeNil())
		})

		It("Should return error for unknown encoding", func() {
			event := NewTransactionEvent("package", "nginx", "")

			_, err := MarshalResourceEnsureApiResponse(UnknownRequestEncoding, event, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported encoding"))
		})

		It("Should return error for invalid encoding value", func() {
			event := NewTransactionEvent("package", "nginx", "")

			_, err := MarshalResourceEnsureApiResponse(RequestEncoding(99), event, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported encoding"))
		})
	})

	Describe("ResourceEnsureApiRequest struct", func() {
		It("Should unmarshal properties correctly from JSON", func() {
			jsonReq := `{
				"protocol": "io.choria.ccm.v1.resource.ensure.request",
				"type": "package",
				"properties": {"name": "nginx", "ensure": "present", "version": "1.2.3"}
			}`

			req, err := UnmarshalResourceEnsureApiRequest([]byte(jsonReq))
			Expect(err).ToNot(HaveOccurred())

			var props map[string]any
			err = yaml.Unmarshal(req.Properties, &props)
			Expect(err).ToNot(HaveOccurred())
			Expect(props["name"]).To(Equal("nginx"))
			Expect(props["ensure"]).To(Equal("present"))
			Expect(props["version"]).To(Equal("1.2.3"))
		})

		It("Should unmarshal nested properties from YAML", func() {
			yamlReq := `
protocol: io.choria.ccm.v1.resource.ensure.request
type: file
properties:
  name: /etc/test.conf
  ensure: present
  content: |
    line1
    line2
`
			req, err := UnmarshalResourceEnsureApiRequest([]byte(yamlReq))
			Expect(err).ToNot(HaveOccurred())

			var props map[string]any
			err = yaml.Unmarshal(req.Properties, &props)
			Expect(err).ToNot(HaveOccurred())
			Expect(props["name"]).To(Equal("/etc/test.conf"))
			Expect(props["content"]).To(ContainSubstring("line1"))
		})
	})

	Describe("ResourceEnsureApiResponse struct", func() {
		It("Should have correct structure with event", func() {
			event := NewTransactionEvent("package", "nginx", "")
			data, err := MarshalResourceEnsureApiResponse(JsonRequestEncoding, event, nil)
			Expect(err).ToNot(HaveOccurred())

			var resp map[string]any
			err = json.Unmarshal(data, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp["protocol"]).To(Equal(ResourceEnsureApiResponseProtocol))
			Expect(resp["state"]).ToNot(BeNil())
			// Top-level error should not be present when no error
			_, hasError := resp["error"]
			Expect(hasError).To(BeFalse())
		})

		It("Should omit state field when nil in JSON", func() {
			testErr := errors.New("test error")
			data, err := MarshalResourceEnsureApiResponse(JsonRequestEncoding, nil, testErr)
			Expect(err).ToNot(HaveOccurred())

			var resp map[string]any
			err = json.Unmarshal(data, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp["error"]).To(Equal("test error"))
			// State should not be present when there's an error
			_, hasState := resp["state"]
			Expect(hasState).To(BeFalse())
		})
	})
})
