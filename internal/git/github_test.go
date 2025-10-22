package git

import (
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "HTTPS URL with .git",
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL without .git",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL with credentials and .git",
			url:       "https://user:pass@github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL with credentials without .git",
			url:       "https://user:pass@github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "SSH URL",
			url:       "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL with credentials for enterprise GitHub",
			url:       "https://mapa:github_personal_access_token@git.supercorp.com/project/cluster",
			wantOwner: "project",
			wantRepo:  "cluster",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL with credentials for enterprise GitHub with .git",
			url:       "https://user:token@git.example.com/org/project.git",
			wantOwner: "org",
			wantRepo:  "project",
			wantErr:   false,
		},
		{
			name:      "Invalid URL",
			url:       "invalid-url",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseGitHubURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("parseGitHubURL() owner = %v, want %v", owner, tt.wantOwner)
			}

			if repo != tt.wantRepo {
				t.Errorf("parseGitHubURL() repo = %v, want %v", repo, tt.wantRepo)
			}
		})
	}
}

func TestExtractAPIBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		want    string
	}{
		{
			name:    "GitHub.com HTTPS",
			repoURL: "https://github.com/owner/repo.git",
			want:    "https://api.github.com",
		},
		{
			name:    "GitHub.com HTTPS with credentials",
			repoURL: "https://user:token@github.com/owner/repo.git",
			want:    "https://api.github.com",
		},
		{
			name:    "GitHub.com SSH",
			repoURL: "git@github.com:owner/repo.git",
			want:    "https://api.github.com",
		},
		{
			name:    "Enterprise GitHub HTTPS",
			repoURL: "https://git.example.com/owner/repo.git",
			want:    "https://git.example.com/api/v3",
		},
		{
			name:    "Enterprise GitHub HTTPS with credentials",
			repoURL: "https://user:token@git.supercorp.com/project/cluster",
			want:    "https://git.supercorp.com/api/v3",
		},
		{
			name:    "Enterprise GitHub SSH",
			repoURL: "git@git.example.com:owner/repo.git",
			want:    "https://git.example.com/api/v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAPIBaseURL(tt.repoURL)
			if got != tt.want {
				t.Errorf("extractAPIBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
