package shared

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

// BrowserConfig holds configuration for browser operations
type BrowserConfig struct {
	AllowedDomains []string
	MaxLinksPerOp  int
}

// DefaultBrowserConfig returns a safe default configuration
func DefaultBrowserConfig() *BrowserConfig {
	return &BrowserConfig{
		AllowedDomains: []string{
			"konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com",
			"art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com",
			"github.com",
			"api.github.com",
		},
		MaxLinksPerOp: 10,
	}
}

// BrowserOpenResult represents the result of opening a browser link
type BrowserOpenResult struct {
	URL     string `json:"url"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BrowserOpenResponse represents the response from opening multiple links
type BrowserOpenResponse struct {
	Results     []BrowserOpenResult `json:"results"`
	TotalOpened int                 `json:"totalOpened"`
	TotalFailed int                 `json:"totalFailed"`
}

// ValidateURL checks if a URL is allowed based on domain whitelist
func (config *BrowserConfig) ValidateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP/HTTPS URLs are allowed")
	}

	hostname := parsedURL.Hostname()
	for _, allowedDomain := range config.AllowedDomains {
		if hostname == allowedDomain || strings.HasSuffix(hostname, "."+allowedDomain) {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in allowed list", hostname)
}

// OpenBrowserLink opens a single URL in the default browser
func OpenBrowserLink(urlStr string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "linux":
		cmd = exec.Command("xdg-open", urlStr)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// OpenBrowserLinks opens multiple URLs with validation and safety checks
func OpenBrowserLinks(urls []string, config *BrowserConfig) *BrowserOpenResponse {
	if config == nil {
		config = DefaultBrowserConfig()
	}

	response := &BrowserOpenResponse{
		Results: make([]BrowserOpenResult, 0, len(urls)),
	}

	if len(urls) > config.MaxLinksPerOp {
		for i, urlStr := range urls {
			if i >= config.MaxLinksPerOp {
				response.Results = append(response.Results, BrowserOpenResult{
					URL:     urlStr,
					Success: false,
					Error:   fmt.Sprintf("exceeded maximum links per operation (%d)", config.MaxLinksPerOp),
				})
				response.TotalFailed++
				continue
			}

			result := openSingleLink(urlStr, config)
			response.Results = append(response.Results, result)
			if result.Success {
				response.TotalOpened++
			} else {
				response.TotalFailed++
			}
		}
		return response
	}

	for _, urlStr := range urls {
		result := openSingleLink(urlStr, config)
		response.Results = append(response.Results, result)
		if result.Success {
			response.TotalOpened++
		} else {
			response.TotalFailed++
		}
	}

	return response
}

// openSingleLink handles opening a single link with validation
func openSingleLink(urlStr string, config *BrowserConfig) BrowserOpenResult {
	result := BrowserOpenResult{URL: urlStr}

	if err := config.ValidateURL(urlStr); err != nil {
		result.Error = fmt.Sprintf("validation failed: %v", err)
		return result
	}

	if err := OpenBrowserLink(urlStr); err != nil {
		result.Error = fmt.Sprintf("failed to open browser: %v", err)
		return result
	}

	result.Success = true
	return result
}
