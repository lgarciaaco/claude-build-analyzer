package shared

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	config := DefaultBrowserConfig()

	tests := []struct {
		url       string
		shouldErr bool
		name      string
	}{
		{
			url:       "https://konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com/ns/test",
			shouldErr: false,
			name:      "valid Konflux URL",
		},
		{
			url:       "https://art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com/job/test",
			shouldErr: false,
			name:      "valid Jenkins URL",
		},
		{
			url:       "https://github.com/openshift/repo",
			shouldErr: false,
			name:      "valid GitHub URL",
		},
		{
			url:       "http://example.com",
			shouldErr: true,
			name:      "invalid domain",
		},
		{
			url:       "ftp://konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com",
			shouldErr: true,
			name:      "invalid protocol",
		},
		{
			url:       "not-a-url",
			shouldErr: true,
			name:      "invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateURL(tt.url)
			if tt.shouldErr && err == nil {
				t.Errorf("expected error for URL %s, got nil", tt.url)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("expected no error for URL %s, got %v", tt.url, err)
			}
		})
	}
}

func TestOpenBrowserLinks(t *testing.T) {
	config := DefaultBrowserConfig()
	config.MaxLinksPerOp = 2 // Set low for testing

	t.Run("valid URLs within limit", func(t *testing.T) {
		urls := []string{
			"https://github.com/test/repo",
		}

		response := OpenBrowserLinks(urls, config)

		if response.TotalFailed > 0 {
			t.Errorf("expected no failures, got %d", response.TotalFailed)
		}

		if len(response.Results) != 1 {
			t.Errorf("expected 1 result, got %d", len(response.Results))
		}
	})

	t.Run("URLs exceeding limit", func(t *testing.T) {
		urls := []string{
			"https://github.com/test/repo1",
			"https://github.com/test/repo2",
			"https://github.com/test/repo3",
		}

		response := OpenBrowserLinks(urls, config)

		if response.TotalFailed == 0 {
			t.Error("expected some failures due to limit, got none")
		}

		if len(response.Results) != 3 {
			t.Errorf("expected 3 results, got %d", len(response.Results))
		}
	})

	t.Run("invalid URLs", func(t *testing.T) {
		urls := []string{
			"http://example.com",
		}

		response := OpenBrowserLinks(urls, config)

		if response.TotalFailed == 0 {
			t.Error("expected failure for invalid domain, got success")
		}

		if len(response.Results) != 1 {
			t.Errorf("expected 1 result, got %d", len(response.Results))
		}

		if response.Results[0].Success {
			t.Error("expected failure for invalid domain, got success")
		}
	})
}

func TestDefaultBrowserConfig(t *testing.T) {
	config := DefaultBrowserConfig()

	if config.MaxLinksPerOp != 10 {
		t.Errorf("expected MaxLinksPerOp to be 10, got %d", config.MaxLinksPerOp)
	}

	if len(config.AllowedDomains) == 0 {
		t.Error("expected at least one allowed domain")
	}

	// Check that required domains are included
	expectedDomains := []string{
		"konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com",
		"art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com",
		"github.com",
	}

	for _, expected := range expectedDomains {
		found := false
		for _, actual := range config.AllowedDomains {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected domain %s not found in allowed domains", expected)
		}
	}
}
