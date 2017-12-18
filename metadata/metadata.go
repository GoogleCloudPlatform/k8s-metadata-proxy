package metadata

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
)

// Attribute key cannot contain special characters or blank spaces. Only
// letters, numbers, underscores (_) and hyphens (-) are allowed.
//
// Service account ID must be between 6 and 30 characters.  Service account ID
// must start with a lower case letter, followed by one or more lower case
// alphanumerical characters that can be separated by hyphens.

// Things that need to not work:
//
//   - /computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip/../../../../../attributes/kube-env
//   - /computeMetadata/v1/instance/attributes//kube-env
//   - /computeMetadata/v1//instance/attributes//kube-env

var (
	concealedEndpoints = []string{
		"/0.1/meta-data/attributes/kube-env",
		"/computeMetadata/v1beta1/instance/attributes/kube-env",
		"/computeMetadata/v1/instance/attributes/kube-env",
	}
	concealedPatterns = []*regexp.Regexp{
		regexp.MustCompile("/0.1/meta-data/service-accounts/.+/identity"),
		regexp.MustCompile("/computeMetadata/v1beta1/instance/service-accounts/.+/identity"),
		regexp.MustCompile("/computeMetadata/v1/instance/service-accounts/.+/identity"),
	}
	knownPrefixes = []string{
		"/0.1/meta-data/",
		"/computeMetadata/v1beta1/",
		"/computeMetadata/v1/",
	}
	discoveryEndpoints = []string{
		"",
		"/",
		"/0.1",
		"/0.1/",
		"/0.1/meta-data",
		"/computeMetadata",
		"/computeMetadata/",
		"/computeMetadata/v1beta1",
		"/computeMetadata/v1",
	}
)

func Filter(req *http.Request) error {
	// Since we're stripping the X-Forwarded-For header that's added by
	// httputil.ReverseProxy.ServeHTTP, check for the header here and
	// refuse to serve if it's present.
	if _, ok := req.Header["X-Forwarded-For"]; ok {
		return errors.New("Calls with X-Forwarded-For header are not allowed by the metadata proxy.")
	}
	// Check that the request isn't a recursive one.
	if req.URL.Query().Get("recursive") != "" {
		return errors.New("?recursive calls are not allowed by the metadata proxy.")
	}

	// Conceal kube-env and vm identity endpoints for known API versions.
	// Don't block unknown API versions, since we don't know if they have
	// the same paths.
	for _, e := range concealedEndpoints {
		if req.URL.Path == e {
			return errors.New("This metadata endpoint is concealed.")
		}
	}
	for _, p := range concealedPatterns {
		if p.MatchString(req.URL.Path) {
			return errors.New("This metadata endpoint is concealed.")
		}
	}

	// Allow proxy for known API versions, defined by prefixes and known
	// discovery endpoints.  Unknown API versions aren't allowed, since we
	// don't know what paths they have.
	for _, p := range knownPrefixes {
		if strings.HasPrefix(req.URL.Path, p) {
			return nil
		}
	}
	for _, e := range discoveryEndpoints {
		if req.URL.Path == e {
			return nil
		}
	}

	// If none of the above checks match, this is an unknown API, so block
	// it.
	return errors.New("This metadata API is not allowed by the metadata proxy.")
}
