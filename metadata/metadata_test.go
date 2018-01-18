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
	recursiveErr     = errors.New("?recursive calls are not allowed by the metadata proxy.")
	xffErr           = errors.New("Calls with X-Forwarded-For header are not allowed by the metadata proxy.")
	parseErr         = errors.New("Metadata proxy could not safely parse request.")
)

func TestFilterURL(t *testing.T) {
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
		// Params that contain 'recursive' as substring.
		{"/computeMetadata/v1/instance/?nonrecursive=true", nil, "/computeMetadata/v1/instance/"},
		{"/computeMetadata/v1/instance/?something=other&nonrecursive=true", nil, "/computeMetadata/v1/instance/"},

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
		// Recursive.
		{"/computeMetadata/v1/instance/?recursive=true", recursiveErr, ""},
		{"/computeMetadata/v1/instance/?something=other&recursive=true", recursiveErr, ""},
		{"/computeMetadata/v1/instance/?recursive=true&something=other", recursiveErr, ""},
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
