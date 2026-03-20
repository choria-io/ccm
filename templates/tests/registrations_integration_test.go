// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package tests_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/templates"
)

func TestRegistrationsIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Templates/RegistrationsIntegration")
}

var _ = Describe("RegistrationEntries in templates", func() {
	var env *templates.Env

	BeforeEach(func() {
		env = &templates.Env{
			Facts: map[string]any{},
			Data:  map[string]any{},
		}
		env.RegistrationsFunc = func(cluster, protocol, service, ip string) (any, error) {
			return model.RegistrationEntries{
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.1", Port: int64(8080)},
				{Cluster: "dev", Service: "prometheus", Protocol: "tcp", Address: "10.0.0.2", Port: int64(9090)},
			}, nil
		}
	})

	Describe("PrometheusFileSD", func() {
		It("should be callable from expr templates", func() {
			result, err := templates.ExprParse("registrations('dev', 'tcp', 'prometheus', '*').PrometheusFileSD()", env)
			Expect(err).ToNot(HaveOccurred())

			sd, ok := result.(string)
			Expect(ok).To(BeTrue())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(sd), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:8080", "10.0.0.2:9090"}))
		})

		It("should be callable from jet templates", func() {
			result, err := templates.ResolveTemplateString(`{{ jet('[[ registrations("dev", "tcp", "prometheus", "*").PrometheusFileSD() ]]') }}`, env)
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal([]byte(result), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:8080", "10.0.0.2:9090"}))
		})

		It("should be callable from go templates", func() {
			goTpl := `{{ $r := registrations "dev" "tcp" "prometheus" "*" }}{{ $r.PrometheusFileSD }}`
			tmpl, err := template.New("test").Funcs(env.GoFunctions()).Parse(goTpl)
			Expect(err).ToNot(HaveOccurred())

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, env)
			Expect(err).ToNot(HaveOccurred())

			var parsed []map[string]any
			Expect(json.Unmarshal(buf.Bytes(), &parsed)).To(Succeed())
			Expect(parsed).To(HaveLen(1))
			Expect(parsed[0]["targets"]).To(Equal([]any{"10.0.0.1:8080", "10.0.0.2:9090"}))
		})
	})
})
