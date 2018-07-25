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
		"/computeMetadata/v1beta1/instance/attributes/kube-env",
		"/computeMetadata/v1/instance/attributes/kube-env",
	}
	concealedPatterns = []*regexp.Regexp{
		regexp.MustCompile("/0.1/meta-data/service-accounts/.+/identity"),
		regexp.MustCompile("/computeMetadata/v1beta1/instance/service-accounts/.+/identity"),
		regexp.MustCompile("/computeMetadata/v1/instance/service-accounts/.+/identity"),
	}
	recursiveWhitelistPatterns = []*regexp.Regexp{
		// ?recursive=true on the instance service account metadata returns
		// `aliases`, `email`, and `scopes` for the specified service account, none
		// of which are concealed. This is used by GCE's python oauth2 lib to
		// fetch access_tokens.
		regexp.MustCompile("/0.1/meta-data/service-accounts/.+/"),
		regexp.MustCompile("/computeMetadata/v1beta1/instance/service-accounts/.+/"),
		regexp.MustCompile("/computeMetadata/v1/instance/service-accounts/.+/"),
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
	knownPrefixes = []string{
		"/0.1/meta-data/",
		"/computeMetadata/v1beta1/",
		"/computeMetadata/v1/",
	}
)

// whitelistedRecursiveEndpoint returns whether or not the given path
// corresponds to an endpoint which has been whitelisted for ?recursive calls.
func whitelistedRecursiveEndpoint(path string) bool {
	for _, p := range recursiveWhitelistPatterns {
		if p.MatchString(path) {
			return true
		}
	}
	return false
}

// Filter returns a cleaned path if the request ought to be allowed, or an
// error if not.
func Filter(req *http.Request) (string, error) {
	// Since we're stripping the X-Forwarded-For header that's added by
	// httputil.ReverseProxy.ServeHTTP, check for the header here and
	// refuse to serve if it's present.
	if _, ok := req.Header["X-Forwarded-For"]; ok {
		return "", errors.New("Calls with X-Forwarded-For header are not allowed by the metadata proxy.")
	}

	// Check that the request doesn't have any opaque parts.
	if req.URL.Opaque != "" {
		return "", errors.New("Metadata proxy could not safely parse request.")
	}

	cleanedPath := path.Clean(req.URL.Path)
	// path.Clean("") == ".", so set it back to "".
	if req.URL.Path == "" {
		cleanedPath = ""
	}
	// path.Clean("/") == "/", so set it to "" and append "/" below.
	if req.URL.Path == "/" {
		cleanedPath = ""
	}
	if strings.HasSuffix(req.URL.Path, "/") && cleanedPath != "/" {
		cleanedPath += "/"
	}

	// Check that the request isn't a recursive one, or has been whitelisted.
	if req.URL.Query().Get("recursive") != "" && !whitelistedRecursiveEndpoint(cleanedPath) {
		return "", errors.New("This metadata endpoint is concealed for ?recursive calls.")
	}

	// Conceal kube-env and vm identity endpoints for known API versions.
	// Don't block unknown API versions, since we don't know if they have
	// the same paths.
	for _, e := range concealedEndpoints {
		if cleanedPath == e {
			return "", errors.New("This metadata endpoint is concealed.")
		}
	}
	for _, p := range concealedPatterns {
		if p.MatchString(cleanedPath) {
			return "", errors.New("This metadata endpoint is concealed.")
		}
	}

	// Allow known discovery endpoints.
	for _, e := range discoveryEndpoints {
		if cleanedPath == e {
			return cleanedPath, nil
		}
	}
	// Allow proxy for known API versions, defined by prefixes and known
	// discovery endpoints.  Unknown API versions aren't allowed, since we
	// don't know what paths they have.
	for _, p := range knownPrefixes {
		if strings.HasPrefix(cleanedPath, p) {
			return cleanedPath, nil
		}
	}

	// If none of the above checks match, this is an unknown API, so block
	// it.
	return "", errors.New("This metadata API is not allowed by the metadata proxy.")
}
