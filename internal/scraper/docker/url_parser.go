package docker

import (
	"fmt"
	"strings"
)

// ImageInfo contains parsed Docker image information
type ImageInfo struct {
	Registry   string // e.g., "gcr.io", "docker.io", or empty for Docker Hub
	Repository string // e.g., "library/nginx", "myorg/myapp"
	Tag        string // e.g., "latest", "1.2.3" (if specified in URI)
}

// ParseImageURL extracts registry, repository, and optional tag from a Docker image URL
// Supports multiple URI formats:
// - nginx (Docker Hub, library namespace)
// - nginx:1.21 (Docker Hub with tag)
// - myorg/myapp (Docker Hub, custom namespace)
// - myorg/myapp:v1.2.3 (Docker Hub with tag)
// - gcr.io/myproject/myapp (Google Container Registry)
// - registry.example.com/myorg/myapp:latest (Custom registry)
// - registry.example.com:5000/myorg/myapp (Custom registry with port)
func ParseImageURL(uri string) (*ImageInfo, error) {
	if uri == "" {
		return nil, fmt.Errorf("empty image URI provided")
	}

	// Remove common prefixes
	uri = strings.TrimPrefix(uri, "docker://")
	uri = strings.TrimPrefix(uri, "https://")
	uri = strings.TrimPrefix(uri, "http://")

	// Split by tag separator if present
	var tag string
	if idx := strings.LastIndex(uri, ":"); idx != -1 {
		// Check if this is a port number or a tag
		// Port numbers come after the registry, tags come after the repository
		possibleTag := uri[idx+1:]
		if !strings.Contains(possibleTag, "/") {
			// It's either a port or a tag
			// If there's a slash before the colon, it's likely a tag
			// OR if there's no slash at all (single name with tag like "nginx:1.21")
			beforeColon := uri[:idx]
			if strings.Contains(beforeColon, "/") || !strings.Contains(beforeColon, ".") {
				// Has slashes, or no dots (not a domain), so this is likely a tag
				tag = possibleTag
				uri = beforeColon
			}
		}
	}

	// Determine if there's a registry
	var registry string
	var repository string

	parts := strings.Split(uri, "/")

	if len(parts) == 1 {
		// Single part: "nginx" -> Docker Hub, library namespace
		registry = ""
		repository = "library/" + parts[0]
	} else if len(parts) == 2 {
		// Two parts: check if first part is a registry or namespace
		firstPart := parts[0]

		// If first part contains a dot, colon, or is "localhost", it's a registry
		if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
			registry = firstPart
			repository = parts[1]
		} else {
			// It's a Docker Hub namespace
			registry = ""
			repository = uri
		}
	} else {
		// Three or more parts: first is registry, rest is repository
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	}

	// Normalize Docker Hub registry
	if registry == "docker.io" || registry == "index.docker.io" {
		registry = ""
	}

	if repository == "" {
		return nil, fmt.Errorf("invalid Docker image URI: %s (no repository found)", uri)
	}

	return &ImageInfo{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
	}, nil
}

// BuildRegistryURL constructs the appropriate Docker registry URL
// For Docker Hub, returns the Docker Hub registry URL
// For custom registries, uses the provided base URL or the registry from the image
func BuildRegistryURL(baseURL string, imageRegistry string) string {
	// If a base URL is explicitly configured, use it
	if baseURL != "" {
		return strings.TrimSuffix(baseURL, "/")
	}

	// If the image specifies a registry, use it
	if imageRegistry != "" {
		// Add https:// if not present
		if !strings.HasPrefix(imageRegistry, "http://") && !strings.HasPrefix(imageRegistry, "https://") {
			return "https://" + imageRegistry
		}
		return imageRegistry
	}

	// Default to Docker Hub registry
	return "https://registry.hub.docker.com"
}
