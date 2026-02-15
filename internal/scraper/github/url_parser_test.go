package github

import (
	"testing"
)

func TestParseRepositoryURL(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantOwner   string
		wantRepo    string
		wantErr     bool
		errContains string
	}{
		{
			name:      "public github - basic",
			uri:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "public github - with .git suffix",
			uri:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "public github api - repos path",
			uri:       "https://api.github.com/repos/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "public github api - with releases/latest",
			uri:       "https://api.github.com/repos/owner/repo/releases/latest",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "public github api - with releases",
			uri:       "https://api.github.com/repos/owner/repo/releases",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "github enterprise - basic",
			uri:       "https://github.enterprise.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "github enterprise - with .git",
			uri:       "https://github.enterprise.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "github enterprise api - with api/v3",
			uri:       "https://github.enterprise.com/api/v3/repos/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "github enterprise api - with releases/latest",
			uri:       "https://github.enterprise.com/api/v3/repos/owner/repo/releases/latest",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "http protocol",
			uri:       "http://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "trailing slash",
			uri:       "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "hyphenated owner and repo",
			uri:       "https://github.com/my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
			wantErr:   false,
		},
		{
			name:      "underscored owner and repo",
			uri:       "https://github.com/my_org/my_repo",
			wantOwner: "my_org",
			wantRepo:  "my_repo",
			wantErr:   false,
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "empty URI",
		},
		{
			name:        "no path",
			uri:         "https://github.com",
			wantErr:     true,
			errContains: "no path found",
		},
		{
			name:        "only owner no repo",
			uri:         "https://github.com/owner",
			wantErr:     true,
			errContains: "expected format: owner/repo",
		},
		{
			name:        "invalid format",
			uri:         "not-a-url",
			wantErr:     true,
			errContains: "no path found",
		},
		{
			name:        "empty owner",
			uri:         "https://github.com//repo",
			wantErr:     true,
			errContains: "owner or repo is empty",
		},
		{
			name:      "raw.githubusercontent.com URL",
			uri:       "https://raw.githubusercontent.com/traefik/traefik-helm-chart/refs/heads/master/traefik/Chart.yaml",
			wantOwner: "traefik",
			wantRepo:  "traefik-helm-chart",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRepositoryURL(tt.uri)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRepositoryURL() error = nil, wantErr = true")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseRepositoryURL() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRepositoryURL() unexpected error = %v", err)
				return
			}

			if got.Owner != tt.wantOwner {
				t.Errorf("ParseRepositoryURL() owner = %v, want %v", got.Owner, tt.wantOwner)
			}

			if got.Repo != tt.wantRepo {
				t.Errorf("ParseRepositoryURL() repo = %v, want %v", got.Repo, tt.wantRepo)
			}
		})
	}
}

func TestBuildAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "empty base url - defaults to public github",
			baseURL: "",
			want:    "https://api.github.com",
		},
		{
			name:    "github enterprise without api path",
			baseURL: "https://github.enterprise.com",
			want:    "https://github.enterprise.com/api/v3",
		},
		{
			name:    "github enterprise with trailing slash",
			baseURL: "https://github.enterprise.com/",
			want:    "https://github.enterprise.com/api/v3",
		},
		{
			name:    "github enterprise with api/v3 already present",
			baseURL: "https://github.enterprise.com/api/v3",
			want:    "https://github.enterprise.com/api/v3",
		},
		{
			name:    "github enterprise with api/v3 and trailing slash",
			baseURL: "https://github.enterprise.com/api/v3/",
			want:    "https://github.enterprise.com/api/v3",
		},
		{
			name:    "custom port",
			baseURL: "https://github.company.local:8443",
			want:    "https://github.company.local:8443/api/v3",
		},
		{
			name:    "http protocol",
			baseURL: "http://github.internal",
			want:    "http://github.internal/api/v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildAPIURL(tt.baseURL)
			if got != tt.want {
				t.Errorf("BuildAPIURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
