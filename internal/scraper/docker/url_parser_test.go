package docker

import (
	"testing"
)

func TestParseImageURL(t *testing.T) {
	tests := []struct {
		name           string
		uri            string
		wantRegistry   string
		wantRepository string
		wantTag        string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "docker hub - single name",
			uri:            "nginx",
			wantRegistry:   "",
			wantRepository: "library/nginx",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "docker hub - single name with tag",
			uri:            "nginx:1.21",
			wantRegistry:   "",
			wantRepository: "library/nginx",
			wantTag:        "1.21",
			wantErr:        false,
		},
		{
			name:           "docker hub - org/repo",
			uri:            "myorg/myapp",
			wantRegistry:   "",
			wantRepository: "myorg/myapp",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "docker hub - org/repo with tag",
			uri:            "myorg/myapp:v1.2.3",
			wantRegistry:   "",
			wantRepository: "myorg/myapp",
			wantTag:        "v1.2.3",
			wantErr:        false,
		},
		{
			name:           "docker hub - explicit docker.io",
			uri:            "docker.io/library/nginx",
			wantRegistry:   "",
			wantRepository: "library/nginx",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "docker hub - index.docker.io",
			uri:            "index.docker.io/library/nginx:latest",
			wantRegistry:   "",
			wantRepository: "library/nginx",
			wantTag:        "latest",
			wantErr:        false,
		},
		{
			name:           "gcr - google container registry",
			uri:            "gcr.io/myproject/myapp",
			wantRegistry:   "gcr.io",
			wantRepository: "myproject/myapp",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "gcr - with tag",
			uri:            "gcr.io/myproject/myapp:v1.0.0",
			wantRegistry:   "gcr.io",
			wantRepository: "myproject/myapp",
			wantTag:        "v1.0.0",
			wantErr:        false,
		},
		{
			name:           "custom registry - with domain",
			uri:            "registry.example.com/myorg/myapp",
			wantRegistry:   "registry.example.com",
			wantRepository: "myorg/myapp",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "custom registry - with port",
			uri:            "registry.example.com:5000/myorg/myapp",
			wantRegistry:   "registry.example.com:5000",
			wantRepository: "myorg/myapp",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "custom registry - with port and tag",
			uri:            "registry.example.com:5000/myorg/myapp:latest",
			wantRegistry:   "registry.example.com:5000",
			wantRepository: "myorg/myapp",
			wantTag:        "latest",
			wantErr:        false,
		},
		{
			name:           "localhost registry",
			uri:            "localhost:5000/myapp",
			wantRegistry:   "localhost:5000",
			wantRepository: "myapp",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "localhost registry with tag",
			uri:            "localhost:5000/myapp:dev",
			wantRegistry:   "localhost:5000",
			wantRepository: "myapp",
			wantTag:        "dev",
			wantErr:        false,
		},
		{
			name:           "nested repository path",
			uri:            "gcr.io/myproject/team/app",
			wantRegistry:   "gcr.io",
			wantRepository: "myproject/team/app",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "docker:// prefix",
			uri:            "docker://nginx",
			wantRegistry:   "",
			wantRepository: "library/nginx",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:           "https:// prefix",
			uri:            "https://registry.example.com/myapp",
			wantRegistry:   "registry.example.com",
			wantRepository: "myapp",
			wantTag:        "",
			wantErr:        false,
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "empty image URI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageURL(tt.uri)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseImageURL() error = nil, wantErr = true")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseImageURL() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseImageURL() unexpected error = %v", err)
				return
			}

			if got.Registry != tt.wantRegistry {
				t.Errorf("ParseImageURL() registry = %v, want %v", got.Registry, tt.wantRegistry)
			}

			if got.Repository != tt.wantRepository {
				t.Errorf("ParseImageURL() repository = %v, want %v", got.Repository, tt.wantRepository)
			}

			if got.Tag != tt.wantTag {
				t.Errorf("ParseImageURL() tag = %v, want %v", got.Tag, tt.wantTag)
			}
		})
	}
}

func TestBuildRegistryURL(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		imageRegistry string
		want          string
	}{
		{
			name:          "explicit base url",
			baseURL:       "https://my-registry.com",
			imageRegistry: "gcr.io",
			want:          "https://my-registry.com",
		},
		{
			name:          "base url with trailing slash",
			baseURL:       "https://my-registry.com/",
			imageRegistry: "",
			want:          "https://my-registry.com",
		},
		{
			name:          "image registry with https",
			baseURL:       "",
			imageRegistry: "https://gcr.io",
			want:          "https://gcr.io",
		},
		{
			name:          "image registry without protocol",
			baseURL:       "",
			imageRegistry: "gcr.io",
			want:          "https://gcr.io",
		},
		{
			name:          "image registry with port",
			baseURL:       "",
			imageRegistry: "registry.example.com:5000",
			want:          "https://registry.example.com:5000",
		},
		{
			name:          "empty - defaults to docker hub",
			baseURL:       "",
			imageRegistry: "",
			want:          "https://registry.hub.docker.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRegistryURL(tt.baseURL, tt.imageRegistry)
			if got != tt.want {
				t.Errorf("BuildRegistryURL() = %v, want %v", got, tt.want)
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
