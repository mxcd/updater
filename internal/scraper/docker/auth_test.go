package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxcd/updater/internal/configuration"
)

func TestParseWwwAuthenticate(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantRealm string
		wantSvc   string
		wantScope string
		wantErr   bool
	}{
		{
			name:      "ghcr.io standard header",
			header:    `Bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:myorg/myimage:pull"`,
			wantRealm: "https://ghcr.io/token",
			wantSvc:   "ghcr.io",
			wantScope: "repository:myorg/myimage:pull",
		},
		{
			name:      "realm only",
			header:    `Bearer realm="https://auth.example.com/token"`,
			wantRealm: "https://auth.example.com/token",
		},
		{
			name:      "with extra spaces",
			header:    `Bearer  realm="https://ghcr.io/token" , service="ghcr.io" , scope="repository:org/img:pull"`,
			wantRealm: "https://ghcr.io/token",
			wantSvc:   "ghcr.io",
			wantScope: "repository:org/img:pull",
		},
		{
			name:    "non-Bearer scheme",
			header:  `Basic realm="registry"`,
			wantErr: true,
		},
		{
			name:    "missing realm",
			header:  `Bearer service="ghcr.io"`,
			wantErr: true,
		},
		{
			name:    "empty header",
			header:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge, err := parseWwwAuthenticate(tt.header)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if challenge.Realm != tt.wantRealm {
				t.Errorf("realm = %q, want %q", challenge.Realm, tt.wantRealm)
			}
			if challenge.Service != tt.wantSvc {
				t.Errorf("service = %q, want %q", challenge.Service, tt.wantSvc)
			}
			if challenge.Scope != tt.wantScope {
				t.Errorf("scope = %q, want %q", challenge.Scope, tt.wantScope)
			}
		})
	}
}

func TestGetNextPageURL(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		baseURL  string
		wantURL  string
	}{
		{
			name:    "relative URL",
			header:  `</v2/myorg/myimage/tags/list?n=100&last=v1.0.0>; rel="next"`,
			baseURL: "https://ghcr.io",
			wantURL: "https://ghcr.io/v2/myorg/myimage/tags/list?n=100&last=v1.0.0",
		},
		{
			name:    "absolute URL",
			header:  `<https://ghcr.io/v2/myorg/myimage/tags/list?n=100&last=v1.0.0>; rel="next"`,
			baseURL: "https://ghcr.io",
			wantURL: "https://ghcr.io/v2/myorg/myimage/tags/list?n=100&last=v1.0.0",
		},
		{
			name:    "empty header",
			header:  "",
			baseURL: "https://ghcr.io",
			wantURL: "",
		},
		{
			name:    "no next rel",
			header:  `</v2/myorg/myimage/tags/list?n=100>; rel="prev"`,
			baseURL: "https://ghcr.io",
			wantURL: "",
		},
		{
			name:    "multiple links with next",
			header:  `</v2/repo/tags/list?last=a>; rel="prev", </v2/repo/tags/list?n=100&last=b>; rel="next"`,
			baseURL: "https://registry.example.com",
			wantURL: "https://registry.example.com/v2/repo/tags/list?n=100&last=b",
		},
		{
			name:    "base URL with trailing slash",
			header:  `</v2/repo/tags/list?n=100>; rel="next"`,
			baseURL: "https://ghcr.io/",
			wantURL: "https://ghcr.io/v2/repo/tags/list?n=100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNextPageURL(tt.header, tt.baseURL)
			if got != tt.wantURL {
				t.Errorf("getNextPageURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestDoAuthenticatedRequest_DirectSuccess(t *testing.T) {
	// Server returns 200 directly — no auth challenge needed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "myorg/myimage",
			"tags": []string{"v1.0.0", "v1.1.0"},
		})
	}))
	defer server.Close()

	provider := &configuration.PackageSourceProvider{
		AuthType: configuration.PackageSourceProviderAuthTypeNone,
	}

	client := server.Client()
	resp, err := doAuthenticatedRequest(client, server.URL+"/v2/myorg/myimage/tags/list", provider, "myorg/myimage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDoAuthenticatedRequest_401ThenTokenExchange(t *testing.T) {
	callCount := 0

	// Combined server: handles both registry and token endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/myorg/myimage/tags/list":
			callCount++
			if callCount == 1 {
				// First call: return 401 with challenge pointing to our own /token
				w.Header().Set("Www-Authenticate", fmt.Sprintf(
					`Bearer realm="%s/token",service="test-registry",scope="repository:myorg/myimage:pull"`,
					"http://"+r.Host,
				))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// Second call (after token exchange): verify Bearer token
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token-123" {
				t.Errorf("retry request auth = %q, want %q", auth, "Bearer test-token-123")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "myorg/myimage",
				"tags": []string{"v1.0.0"},
			})

		case "/token":
			// Verify credentials were sent as basic auth
			user, pass, ok := r.BasicAuth()
			if !ok || user != "token" || pass != "my-pat" {
				t.Errorf("token endpoint got basic auth: user=%q pass=%q ok=%v", user, pass, ok)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token": "test-token-123",
			})
		}
	}))
	defer server.Close()

	provider := &configuration.PackageSourceProvider{
		AuthType: configuration.PackageSourceProviderAuthTypeToken,
		Token:    "my-pat",
	}

	client := server.Client()
	resp, err := doAuthenticatedRequest(client, server.URL+"/v2/myorg/myimage/tags/list", provider, "myorg/myimage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("expected 2 registry calls (initial + retry), got %d", callCount)
	}
}

func TestDoAuthenticatedRequest_401NoWwwAuthenticate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := &configuration.PackageSourceProvider{
		AuthType: configuration.PackageSourceProviderAuthTypeNone,
	}

	client := server.Client()
	_, err := doAuthenticatedRequest(client, server.URL+"/v2/myorg/myimage/tags/list", provider, "myorg/myimage")
	if err == nil {
		t.Fatal("expected error for 401 without Www-Authenticate")
	}
}

func TestDoAuthenticatedRequest_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/repo/tags/list":
			w.Header().Set("Www-Authenticate", fmt.Sprintf(
				`Bearer realm="%s/token",service="registry"`,
				"http://"+r.Host,
			))
			w.WriteHeader(http.StatusUnauthorized)

		case "/token":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "myuser" || pass != "mypass" {
				t.Errorf("token endpoint got basic auth: user=%q pass=%q ok=%v", user, pass, ok)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token": "basic-token",
			})
		}
	}))
	defer server.Close()

	provider := &configuration.PackageSourceProvider{
		AuthType: configuration.PackageSourceProviderAuthTypeBasic,
		Username: "myuser",
		Password: "mypass",
	}

	client := server.Client()
	// The initial request gets 401, token exchange uses basic auth
	_, err := doAuthenticatedRequest(client, server.URL+"/v2/repo/tags/list", provider, "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchV2TagsPaginated_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "myorg/myimage",
			"tags": []string{"v1.0.0", "v1.1.0", "v2.0.0"},
		})
	}))
	defer server.Close()

	imageInfo := &ImageInfo{Registry: "test.registry.io", Repository: "myorg/myimage"}
	provider := &configuration.PackageSourceProvider{AuthType: configuration.PackageSourceProviderAuthTypeNone}
	source := &configuration.PackageSource{}

	tags, err := fetchV2TagsPaginated(server.URL, imageInfo, provider, source, &ScrapeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("got %d tags, want 3", len(tags))
	}
}

func TestFetchV2TagsPaginated_MultiPage(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case 1:
			w.Header().Set("Link", `</v2/myorg/myimage/tags/list?n=2&last=v1.1.0>; rel="next"`)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "myorg/myimage",
				"tags": []string{"v1.0.0", "v1.1.0"},
			})
		case 2:
			w.Header().Set("Link", `</v2/myorg/myimage/tags/list?n=2&last=v2.1.0>; rel="next"`)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "myorg/myimage",
				"tags": []string{"v2.0.0", "v2.1.0"},
			})
		case 3:
			// Last page, no Link header
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "myorg/myimage",
				"tags": []string{"v3.0.0"},
			})
		}
	}))
	defer server.Close()

	imageInfo := &ImageInfo{Registry: "test.registry.io", Repository: "myorg/myimage"}
	provider := &configuration.PackageSourceProvider{AuthType: configuration.PackageSourceProviderAuthTypeNone}
	source := &configuration.PackageSource{}

	tags, err := fetchV2TagsPaginated(server.URL, imageInfo, provider, source, &ScrapeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 5 {
		t.Errorf("got %d tags, want 5", len(tags))
	}
	if page != 3 {
		t.Errorf("expected 3 pages, got %d", page)
	}
}

func TestFetchV2TagsPaginated_TagLimit(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")

		// Always offer a next page
		w.Header().Set("Link", `</v2/myorg/myimage/tags/list?n=3&last=next>; rel="next"`)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "myorg/myimage",
			"tags": []string{fmt.Sprintf("v%d.0.0", page), fmt.Sprintf("v%d.1.0", page), fmt.Sprintf("v%d.2.0", page)},
		})
	}))
	defer server.Close()

	imageInfo := &ImageInfo{Registry: "test.registry.io", Repository: "myorg/myimage"}
	provider := &configuration.PackageSourceProvider{AuthType: configuration.PackageSourceProviderAuthTypeNone}
	source := &configuration.PackageSource{TagLimit: 5}

	tags, err := fetchV2TagsPaginated(server.URL, imageInfo, provider, source, &ScrapeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) > 5 {
		t.Errorf("got %d tags, want <= 5 (tagLimit)", len(tags))
	}
}

func TestFetchV2TagsPaginated_AuthChallenge(t *testing.T) {
	tokenIssued := false
	registryCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenIssued = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token": "fetched-token",
			})

		default:
			// Registry endpoint
			registryCalls++
			auth := r.Header.Get("Authorization")

			if auth != "Bearer fetched-token" {
				w.Header().Set("Www-Authenticate", fmt.Sprintf(
					`Bearer realm="%s/token",service="test",scope="repository:myorg/myimage:pull"`,
					"http://"+r.Host,
				))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "myorg/myimage",
				"tags": []string{"v1.0.0"},
			})
		}
	}))
	defer server.Close()

	imageInfo := &ImageInfo{Registry: "test.registry.io", Repository: "myorg/myimage"}
	provider := &configuration.PackageSourceProvider{
		AuthType: configuration.PackageSourceProviderAuthTypeToken,
		Token:    "my-pat",
	}
	source := &configuration.PackageSource{}

	tags, err := fetchV2TagsPaginated(server.URL, imageInfo, provider, source, &ScrapeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tokenIssued {
		t.Error("expected token to be issued via exchange")
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags, want 1", len(tags))
	}
}

func TestExchangeForBearerToken_AccessTokenField(t *testing.T) {
	// Some registries return "access_token" instead of "token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "alt-token-456",
		})
	}))
	defer server.Close()

	challenge := &wwwAuthenticateChallenge{
		Realm:   server.URL + "/token",
		Service: "test",
		Scope:   "repo:pull",
	}
	provider := &configuration.PackageSourceProvider{
		AuthType: configuration.PackageSourceProviderAuthTypeNone,
	}

	token, err := exchangeForBearerToken(server.Client(), challenge, provider, "myorg/myimage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "alt-token-456" {
		t.Errorf("token = %q, want %q", token, "alt-token-456")
	}
}
