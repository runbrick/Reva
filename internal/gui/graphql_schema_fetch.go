package gui

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/request"
)

const (
	graphQLSchemaMaxBytes = 512 << 10
	graphQLSchemaDepth    = 8
)

var (
	errGraphQLSchemaEndpoint      = errors.New("missing GraphQL endpoint")
	errGraphQLSchemaAuth          = errors.New("invalid GraphQL authentication")
	errGraphQLSchemaInheritedAuth = errors.New("missing inherited GraphQL authentication")
	errGraphQLSchemaCanceled      = errors.New("GraphQL schema load canceled")
	errGraphQLSchemaLarge         = errors.New("GraphQL schema response exceeds limit")
	errGraphQLSchemaHTTP          = errors.New("GraphQL schema HTTP response was not successful")
	errGraphQLSchemaRejected      = errors.New("GraphQL introspection returned errors")
	errGraphQLSchemaInvalid       = errors.New("invalid GraphQL introspection response")
	errGraphQLSchemaFailed        = errors.New("GraphQL schema transport failed")
	graphQLName                   = regexp.MustCompile(`^[_A-Za-z][_0-9A-Za-z]*$`)
	graphQLSchemaQuery            = `query RevaRootFields {
  __schema {
    queryType { ...RootFields }
    mutationType { ...RootFields }
    subscriptionType { ...RootFields }
  }
}
fragment RootFields on __Type {
  fields(includeDeprecated: true) {
    name
    type { kind name ` + strings.Repeat("ofType { kind name ", graphQLSchemaDepth) + strings.Repeat("}", graphQLSchemaDepth) + ` }
  }
}`
)

type graphQLSchemaJob struct {
	request model.Request
	started atomic.Bool
}

type graphQLSchemaField struct {
	root, signature, search string
}

type graphQLSchemaEnvelope struct {
	Errors []json.RawMessage `json:"errors"`
	Data   struct {
		Schema *struct {
			Query        *graphQLRoot `json:"queryType"`
			Mutation     *graphQLRoot `json:"mutationType"`
			Subscription *graphQLRoot `json:"subscriptionType"`
		} `json:"__schema"`
	} `json:"data"`
}

type graphQLRoot struct {
	Fields []struct {
		Name string         `json:"name"`
		Type graphQLTypeRef `json:"type"`
	} `json:"fields"`
}

type graphQLTypeRef struct {
	Kind   string          `json:"kind"`
	Name   string          `json:"name"`
	OfType *graphQLTypeRef `json:"ofType"`
}

func (ref *graphQLTypeRef) display(depth int) string {
	if ref == nil {
		return ""
	}
	switch ref.Kind {
	case "LIST", "NON_NULL":
		inner := "…"
		if depth > 0 {
			inner = ref.OfType.display(depth - 1)
			if inner == "" {
				return ""
			}
		}
		if ref.Kind == "LIST" {
			return "[" + inner + "]"
		}
		return inner + "!"
	case "SCALAR", "OBJECT", "INTERFACE", "UNION", "ENUM":
		if graphQLName.MatchString(ref.Name) {
			return ref.Name
		}
	}
	return ""
}

func loadGraphQLSchema(ctx context.Context, job *graphQLSchemaJob) ([]graphQLSchemaField, error) {
	if job.started.Swap(true) || ctx.Err() != nil {
		return nil, errGraphQLSchemaCanceled
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = job.request.Options.VerifyTLS == model.ToggleOff
	sender := &request.Service{MaxBodyBytes: graphQLSchemaMaxBytes, Transport: graphQLSchemaTransport{transport}}
	response, err := sender.Send(ctx, graphQLSchemaHTTPRequest(job.request), nil)
	if err != nil {
		return nil, errGraphQLSchemaFailed
	}
	if response.Truncated {
		return nil, errGraphQLSchemaLarge
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, errGraphQLSchemaHTTP
	}
	return parseGraphQLSchema(response.Body)
}

func graphQLSchemaHTTPRequest(req model.Request) model.Request {
	payload, _ := json.Marshal(struct {
		Query string `json:"query"`
	}{graphQLSchemaQuery})
	req.Body = model.Body{Mode: model.BodyJSON, Text: string(payload)}
	req.Options.FollowRedirects = model.ToggleOff
	headers := make([]model.KV, 0, len(req.Headers)+1)
	var hasAccept bool
	for _, row := range req.Headers {
		if row.Disabled || strings.EqualFold(row.Key, "Content-Type") {
			continue
		}
		hasAccept = hasAccept || strings.EqualFold(row.Key, "Accept")
		headers = append(headers, row)
	}
	if !hasAccept {
		headers = append(headers, model.KV{Key: "Accept", Value: "application/graphql-response+json, application/json"})
	}
	req.Headers = headers
	return req
}

type graphQLSchemaTransport struct{ base http.RoundTripper }

func (transport graphQLSchemaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(req)
	if err == nil {
		response.Body = graphQLSchemaLimitedBody{Reader: io.LimitReader(response.Body, graphQLSchemaMaxBytes+1), Closer: response.Body}
	}
	return response, err
}

type graphQLSchemaLimitedBody struct {
	io.Reader
	io.Closer
}

func parseGraphQLSchema(body []byte) ([]graphQLSchemaField, error) {
	var result graphQLSchemaEnvelope
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errGraphQLSchemaInvalid
	}
	if len(result.Errors) != 0 {
		return nil, errGraphQLSchemaRejected
	}
	schema := result.Data.Schema
	if schema == nil || schema.Query == nil {
		return nil, errGraphQLSchemaInvalid
	}
	fields := []graphQLSchemaField{}
	for _, root := range [...]struct {
		label string
		value *graphQLRoot
	}{{"Query", schema.Query}, {"Mutation", schema.Mutation}, {"Subscription", schema.Subscription}} {
		if root.value == nil {
			continue
		}
		if root.value.Fields == nil {
			return nil, errGraphQLSchemaInvalid
		}
		for _, field := range root.value.Fields {
			typeName := field.Type.display(graphQLSchemaDepth)
			if !graphQLName.MatchString(field.Name) || typeName == "" {
				return nil, errGraphQLSchemaInvalid
			}
			signature := field.Name + ": " + typeName
			fields = append(fields, graphQLSchemaField{root.label, signature, strings.ToLower(root.label + " " + signature)})
		}
	}
	return fields, nil
}
