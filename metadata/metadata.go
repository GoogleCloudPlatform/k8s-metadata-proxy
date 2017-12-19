package metadata

import (
	"errors"
	"net/http"
	"path"
	"regexp"
	"strings"
)

var (
	concealedEndpoints = []string{
		"/0.1/meta-data/attributes/kube-env",
		"/computemetadata/v1beta1/instance/attributes/kube-env",
		"/computemetadata/v1/instance/attributes/kube-env",
	}
	concealedPatterns = []*regexp.Regexp{
		regexp.MustCompile("/0.1/meta-data/service-accounts/.+/identity"),
		regexp.MustCompile("/computemetadata/v1beta1/instance/service-accounts/.+/identity"),
		regexp.MustCompile("/computemetadata/v1/instance/service-accounts/.+/identity"),
	}
	discoveryEndpoints = []string{
		".", // path.Clean result for ""
		"/",
		"/0.1",
		"/0.1/meta-data",
		"/computemetadata",
		"/computemetadata/v1beta1",
		"/computemetadata/v1",
	}
	knownPrefixes = []string{
		"/0.1/meta-data/",
		"/computemetadata/v1beta1/",
		"/computemetadata/v1/",
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

	cleanedPath := strings.ToLower(path.Clean(req.URL.Path))

	// Conceal kube-env and vm identity endpoints for known API versions.
	// Don't block unknown API versions, since we don't know if they have
	// the same paths.
	for _, e := range concealedEndpoints {
		if cleanedPath == e {
			return errors.New("This metadata endpoint is concealed.")
		}
	}
	for _, p := range concealedPatterns {
		if p.MatchString(cleanedPath) {
			return errors.New("This metadata endpoint is concealed.")
		}
	}

	// Allow known discovery endpoints.
	for _, e := range discoveryEndpoints {
		if cleanedPath == e {
			return nil
		}
	}
	// Allow proxy for known API versions, defined by prefixes and known
	// discovery endpoints.  Unknown API versions aren't allowed, since we
	// don't know what paths they have.
	for _, p := range knownPrefixes {
		if strings.HasPrefix(cleanedPath, p) {
			return nil
		}
	}

	// If none of the above checks match, this is an unknown API, so block
	// it.
	return errors.New("This metadata API is not allowed by the metadata proxy.")
}
