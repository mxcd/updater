package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

// wwwAuthenticateChallenge holds parsed fields from a Www-Authenticate: Bearer header
type wwwAuthenticateChallenge struct {
	Realm   string
	Service string
	Scope   string
}

// parseWwwAuthenticate parses a Www-Authenticate header value like:
// Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:org/image:pull"
func parseWwwAuthenticate(header string) (*wwwAuthenticateChallenge, error) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, fmt.Errorf("unsupported auth scheme in Www-Authenticate header: %s", header)
	}
	params := header[len("Bearer "):]

	challenge := &wwwAuthenticateChallenge{}
	for _, part := range splitAuthParams(params) {
		part = strings.TrimSpace(part)
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "\"")
		switch strings.ToLower(key) {
		case "realm":
			challenge.Realm = value
		case "service":
			challenge.Service = value
		case "scope":
			challenge.Scope = value
		}
	}

	if challenge.Realm == "" {
		return nil, fmt.Errorf("missing realm in Www-Authenticate header")
	}

	return challenge, nil
}

// splitAuthParams splits comma-separated key=value pairs, respecting quoted values
func splitAuthParams(s string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case r == ',' && !inQuotes:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// exchangeForBearerToken calls the token endpoint from the challenge to get a Bearer token
func exchangeForBearerToken(client *http.Client, challenge *wwwAuthenticateChallenge, provider *configuration.PackageSourceProvider, repository string) (string, error) {
	tokenURL, err := url.Parse(challenge.Realm)
	if err != nil {
		return "", fmt.Errorf("invalid token realm URL: %w", err)
	}

	q := tokenURL.Query()
	if challenge.Service != "" {
		q.Set("service", challenge.Service)
	}
	if challenge.Scope != "" {
		q.Set("scope", challenge.Scope)
	}
	tokenURL.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", tokenURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	// Add credentials based on provider auth type
	switch provider.AuthType {
	case configuration.PackageSourceProviderAuthTypeToken:
		if provider.Token != "" {
			// GitHub PAT pattern: use token as password with "token" username
			req.SetBasicAuth("token", provider.Token)
		}
	case configuration.PackageSourceProviderAuthTypeBasic:
		if provider.Username != "" {
			req.SetBasicAuth(provider.Username, provider.Password)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	// Token response can have "token" or "access_token" field
	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("no token in token exchange response")
	}

	return token, nil
}

// doAuthenticatedRequest makes a GET request with auth challenge handling.
// First tries with static credentials; if 401, exchanges for a Bearer token and retries.
func doAuthenticatedRequest(client *http.Client, requestURL string, provider *configuration.PackageSourceProvider, repository string) (*http.Response, error) {
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Try static auth first
	applyStaticAuth(req, provider)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// If not 401, return the response as-is
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Got 401 — try token exchange via Www-Authenticate challenge
	wwwAuth := resp.Header.Get("Www-Authenticate")
	resp.Body.Close()

	if wwwAuth == "" {
		return nil, fmt.Errorf("received 401 but no Www-Authenticate header")
	}

	log.Debug().Str("www-authenticate", wwwAuth).Msg("received 401 challenge, exchanging for bearer token")

	challenge, err := parseWwwAuthenticate(wwwAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth challenge: %w", err)
	}

	token, err := exchangeForBearerToken(client, challenge, provider, repository)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange for bearer token: %w", err)
	}

	// Retry with the bearer token
	retryReq, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry request: %w", err)
	}
	retryReq.Header.Set("Authorization", "Bearer "+token)

	retryResp, err := client.Do(retryReq)
	if err != nil {
		return nil, fmt.Errorf("retry request failed: %w", err)
	}

	return retryResp, nil
}

// applyStaticAuth sets auth headers on a request based on the provider config
func applyStaticAuth(req *http.Request, provider *configuration.PackageSourceProvider) {
	switch provider.AuthType {
	case configuration.PackageSourceProviderAuthTypeToken:
		if provider.Token != "" {
			req.Header.Set("Authorization", "Bearer "+provider.Token)
		}
	case configuration.PackageSourceProviderAuthTypeBasic:
		if provider.Username != "" {
			req.SetBasicAuth(provider.Username, provider.Password)
		}
	}
}

// getNextPageURL parses the Link header for V2 registry pagination.
// V2 registries return: Link: </v2/repo/tags/list?n=100&last=tag>; rel="next"
func getNextPageURL(linkHeader string, registryBaseURL string) string {
	if linkHeader == "" {
		return ""
	}

	// Parse Link header: </path?params>; rel="next"
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}

		// Extract URL between < and >
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start == -1 || end == -1 || start >= end {
			continue
		}

		linkURL := part[start+1 : end]

		// If it's a relative URL, prepend the registry base URL
		if strings.HasPrefix(linkURL, "/") {
			return strings.TrimSuffix(registryBaseURL, "/") + linkURL
		}

		return linkURL
	}

	return ""
}

// fetchV2TagsPaginated fetches tags from a V2 registry with pagination and auth challenge support
func fetchV2TagsPaginated(registryURL string, imageInfo *ImageInfo, provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]string, error) {
	allTags := make([]string, 0)
	client := &http.Client{Timeout: 30 * time.Second}

	tagLimit := source.TagLimit
	if tagLimit < 0 {
		tagLimit = 0
	}

	pageSize := 100
	nextURL := fmt.Sprintf("%s/v2/%s/tags/list?n=%d", registryURL, imageInfo.Repository, pageSize)
	pageCount := 0

	for nextURL != "" {
		if tagLimit > 0 && len(allTags) >= tagLimit {
			log.Debug().
				Int("tags_fetched", len(allTags)).
				Int("tag_limit", tagLimit).
				Msg("reached tag limit, stopping pagination")
			break
		}

		pageCount++
		log.Trace().
			Str("url", nextURL).
			Int("page", pageCount).
			Msg("fetching V2 registry tags page")

		resp, err := doAuthenticatedRequest(client, nextURL, provider, imageInfo.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tags: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to fetch tags: HTTP %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		linkHeader := resp.Header.Get("Link")
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read tags response: %w", err)
		}

		var tagsResp struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(body, &tagsResp); err != nil {
			return nil, fmt.Errorf("failed to parse registry response: %w", err)
		}

		for _, tag := range tagsResp.Tags {
			if tagLimit > 0 && len(allTags) >= tagLimit {
				break
			}
			allTags = append(allTags, tag)
		}

		nextURL = getNextPageURL(linkHeader, registryURL)

		log.Trace().
			Int("page", pageCount).
			Int("page_tags", len(tagsResp.Tags)).
			Int("total_tags", len(allTags)).
			Bool("has_next", nextURL != "").
			Msg("fetched V2 registry tags page")
	}

	log.Debug().
		Int("total_tags", len(allTags)).
		Int("pages", pageCount).
		Int("tag_limit", tagLimit).
		Bool("limit_reached", tagLimit > 0 && len(allTags) >= tagLimit).
		Msg("finished fetching V2 registry tags")

	return allTags, nil
}
