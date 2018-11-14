package metadata_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-metadata-proxy/metadata"
)

var (
	apiNotAllowedErr = errors.New("This metadata API is not allowed by the metadata proxy.")
	notAllowedErr    = errors.New("This metadata endpoint is not allowed by the metadata proxy.")
	concealedErr     = errors.New("This metadata endpoint is concealed.")
	recursiveErr     = errors.New("This metadata endpoint is concealed for ?recursive calls.")
	xffErr           = errors.New("Calls with X-Forwarded-For header are not allowed by the metadata proxy.")
	parseErr         = errors.New("Metadata proxy could not safely parse request.")
)

func unknownQueryParameterErr(key string) error {
	return fmt.Errorf("Unrecognized query parameter key: %#q.", key)
}

func TestFilterURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url           string
		expectErr     error
		expectCleaned string
	}{
		// Discovery & base.
		{"", nil, ""},
		{"/", nil, "/"},
		{"/0.1", nil, "/0.1"},
		{"/0.1/", nil, "/0.1/"},
		{"/0.1/meta-data", nil, "/0.1/meta-data"},
		{"/0.1/meta-data/", nil, "/0.1/meta-data/"},
		{"/computeMetadata/v1beta1", nil, "/computeMetadata/v1beta1"},
		{"/computeMetadata/v1beta1/", nil, "/computeMetadata/v1beta1/"},
		{"/computeMetadata/v1", nil, "/computeMetadata/v1"},
		{"/computeMetadata/v1/", nil, "/computeMetadata/v1/"},
		// Service account token endpoints.
		{"/computeMetadata/v1/instance/service-accounts/default/token", nil, "/computeMetadata/v1/instance/service-accounts/default/token"},
		{"/computeMetadata/v1/instance/service-accounts/12345-compute@developer.gserviceaccount.com/token", nil, "/computeMetadata/v1/instance/service-accounts/12345-compute@developer.gserviceaccount.com/token"},
		// Service account recursive endpoints (whitelisted).
		{"/computeMetadata/v1/instance/service-accounts/default/?recursive=True", nil, "/computeMetadata/v1/instance/service-accounts/default/"},
		{"/computeMetadata/v1/instance/service-accounts/12345-compute@developer.gserviceaccount.com/?recursive=True", nil, "/computeMetadata/v1/instance/service-accounts/12345-compute@developer.gserviceaccount.com/"},
		// Other known query parameter keys.
		{"/computeMetadata/v1/?alt=text", nil, "/computeMetadata/v1/"},
		{"/computeMetadata/v1/?wait_for_change=true", nil, "/computeMetadata/v1/"},
		{"/computeMetadata/v1/?wait_for_change=true&timeout_sec=3600", nil, "/computeMetadata/v1/"},
		{"/computeMetadata/v1/?wait_for_change=true&last_etag=d34db33fd34db33f", nil, "/computeMetadata/v1/"},
		{"/0.1/meta-data/service-accounts/default/acquire?scopes=cloud-platform+email", nil, "/0.1/meta-data/service-accounts/default/acquire"},
		{"/0.1/meta-data/auth-token?service_account=test@www.example.com&scope=cloud-platform", nil, "/0.1/meta-data/auth-token"},

		// Query params that contain non-whitelisted keys.
		{"/computeMetadata/v1/instance/?nonrecursive=true", unknownQueryParameterErr("nonrecursive"), ""},
		{"/computeMetadata/v1/?something_else=true", unknownQueryParameterErr("something_else"), ""},
		// Other API versions.
		{"/0.2/", apiNotAllowedErr, ""},
		{"/computeMetadata/v2/", apiNotAllowedErr, ""},
		{"/COMPUTEMETADATA/V1/", apiNotAllowedErr, ""},
		// kube-env.
		{"/0.1/meta-data/attributes/kube-env", concealedErr, ""},
		{"/computeMetadata/v1beta1/instance/attributes/kube-env", concealedErr, ""},
		{"/computeMetadata/v1/instance/attributes/kube-env", concealedErr, ""},
		// VM identity.
		{"/0.1/meta-data/service-accounts/default/identity", concealedErr, ""},
		{"/computeMetadata/v1beta1/instance/service-accounts/default/identity", concealedErr, ""},
		{"/computeMetadata/v1/instance/service-accounts/default/identity", concealedErr, ""},
		{"/computeMetadata/v1/instance/service-accounts/default/identity?audience=www.example.com&format=full", concealedErr, ""},
		// Recursive (non-whitelisted).
		{"/computeMetadata/v1/instance/?recursive=true", recursiveErr, ""},
		{"/computeMetadata/v1/instance/?%72%65%63%75%72%73%69%76%65=true", recursiveErr, ""}, // url-hex-encoded
		{"/computeMetadata/v1/instance/?recursive", recursiveErr, ""},
		{"/computeMetadata/v1/instance/?alt=text&recursive=true", recursiveErr, ""},
		{"/computeMetadata/v1/instance/?recursive=true&alt=text", recursiveErr, ""},
		{"/computeMetadata/v1/instance/?alt=text;recursive=true", recursiveErr, ""},
		// Other.
		{"/computeMetadata/v1/instance/attributes//kube-env", concealedErr, ""},
		{"/computeMetadata/v1/instance/attributes/../attributes/kube-env", concealedErr, ""},
		{"opaquescheme:computeMetadata/v1/instance/attributes/kube-env", parseErr, ""},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", tc.url, nil)
			if err != nil {
				t.Fatalf("Unexpected error creating request: %q", err)
			}
			cleanedPath, err := metadata.Filter(req)
			if cleanedPath != tc.expectCleaned {
				t.Errorf("Got cleaned path %q, expected %q", cleanedPath, tc.expectCleaned)
			}
			if err == nil {
				if tc.expectErr != nil {
					t.Errorf("Got nil error, expected %q", tc.expectErr)
				}
			} else if tc.expectErr == nil {
				t.Errorf("Got %q, expected nil error", err)
			} else if err.Error() != tc.expectErr.Error() {
				t.Errorf("Got %q, expected %q", err, tc.expectErr)
			}
		})
	}
}

func TestFilterHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		headers   map[string]string
		expectErr error
	}{
		{map[string]string{}, nil},
		{map[string]string{
			"My-Header": "Hello",
		}, nil},
		{map[string]string{
			"X-Forwarded-For": "That other person",
		}, xffErr},
		{map[string]string{
			"My-Header":       "Hello",
			"X-Forwarded-For": "That other person",
		}, xffErr},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(fmt.Sprintf("%v", tc.headers), func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", "", nil)
			if err != nil {
				t.Fatalf("Unexpected error creating request: %q", err)
			}
			for k, v := range tc.headers {
				req.Header.Add(k, v)
			}
			_, err = metadata.Filter(req)
			if err == nil {
				if tc.expectErr != nil {
					t.Errorf("Got nil error, expected %q", tc.expectErr)
				}
			} else if tc.expectErr == nil {
				t.Errorf("Got %q, expected nil error", err)
			} else if err.Error() != tc.expectErr.Error() {
				t.Errorf("Got %q, expected %q", err, tc.expectErr)
			}
		})
	}
}
