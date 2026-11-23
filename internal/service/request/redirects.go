package request

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/model"
)

const redirectLimit = 10

var errRedirectLimit = errors.New("stopped after 10 redirects")

type redirectTrace struct {
	hops               []model.RedirectHop
	pending            *model.RedirectHop
	pendingAuthRemoved bool
	authKey            string
	lastHadAuth        bool
}

func (trace *redirectTrace) observe(req *http.Request, method model.Method, resp *http.Response, err error, duration time.Duration) {
	if hop := trace.pending; hop != nil {
		hop.NextMethod = method
		hop.Target = diagnosticOrigin(req.URL)
		hop.AuthRemoved = trace.pendingAuthRemoved && !trace.lastHadAuth
		if err == nil && resp != nil {
			hop.Followed, hop.Reason = true, model.RedirectFollowed
		} else {
			hop.Reason = model.RedirectTransportError
		}
		trace.pending, trace.pendingAuthRemoved = nil, false
	}
	if err != nil || resp == nil || len(trace.hops) >= redirectLimit+1 {
		return
	}
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		return
	}
	hop := model.RedirectHop{Method: method, StatusCode: resp.StatusCode, Duration: max(duration, time.Nanosecond), Reason: model.RedirectNotSent}
	if location := resp.Header.Get("Location"); location == "" {
		hop.Reason = model.RedirectMissingTarget
	} else if target, err := req.URL.Parse(location); err != nil {
		hop.Reason = model.RedirectMalformedTarget
	} else {
		hop.Target = diagnosticOrigin(target)
	}
	trace.hops = append(trace.hops, hop)
}

func (trace *redirectTrace) checkRedirect(req *http.Request, err error, disabled, authRemoved bool) {
	if len(trace.hops) == 0 || trace.pending != nil {
		return
	}
	hop := &trace.hops[len(trace.hops)-1]
	if hop.Reason != model.RedirectNotSent || hop.NextMethod != "" {
		return
	}
	hop.NextMethod = diagnosticMethod(req.Method)
	hop.Target = diagnosticOrigin(req.URL)
	switch {
	case err == nil:
		trace.pending, trace.pendingAuthRemoved = hop, authRemoved
	case err == http.ErrUseLastResponse && disabled:
		hop.Reason = model.RedirectDisabled
	case errors.Is(err, errRedirectLimit):
		hop.Reason = model.RedirectLimit
	default:
		hop.Reason = model.RedirectPolicy
	}
}

func (trace *redirectTrace) hasAuth(headers http.Header) bool {
	for key, values := range headers {
		if isRedirectAuthHeader(key, trace.authKey) && hasHeaderValue(values) {
			return true
		}
	}
	return false
}

func isRedirectAuthHeader(key, authKey string) bool {
	return strings.EqualFold(key, "Authorization") || authKey != "" && strings.EqualFold(key, authKey)
}

func hasHeaderValue(values []string) bool {
	for _, value := range values {
		if value != "" {
			return true
		}
	}
	return false
}

func diagnosticMethod(method string) model.Method {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return model.Method(method)
	default:
		return ""
	}
}

func diagnosticOrigin(target *url.URL) string {
	if target == nil || target.Opaque != "" || target.Scheme != "http" && target.Scheme != "https" {
		return ""
	}
	host := target.Hostname()
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Zone() != "" {
			return ""
		}
		host = addr.String()
	} else if !diagnosticHostname(host) {
		return ""
	}
	if port := target.Port(); port != "" {
		if len(port) > 5 {
			return ""
		}
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return ""
		}
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return target.Scheme + "://" + host
}

func diagnosticHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, char := range host {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '.') {
			return false
		}
	}
	return true
}
