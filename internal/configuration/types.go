package configuration

type Config struct {
	PackageSourceProviders []*PackageSourceProvider `yaml:"packageSourceProviders"`
	PackageSources         []*PackageSource         `yaml:"packageSources"`
}

type PackageSourceType string

const (
	PackageSourceTypeGitRelease   PackageSourceType = "git-release"
	PackageSourceTypeGitTag       PackageSourceType = "git-tag"
	PackageSourceTypeGitHelmChart PackageSourceType = "git-helm-chart"
	PackageSourceTypeDockerImage  PackageSourceType = "docker-image"
)

type PackageSource struct {
	Name              string                  `yaml:"name"`
	Provider          string                  `yaml:"provider"`
	Type              PackageSourceType       `yaml:"type"`
	URI               string                  `yaml:"uri"`
	Branch            string                  `yaml:"branch,omitempty"`            // Git branch (for git-helm-chart), defaults to "main"
	Path              string                  `yaml:"path,omitempty"`              // File path in repository (for git-helm-chart)
	VersionConstraint string                  `yaml:"versionConstraint,omitempty"`
	TagPattern        string                  `yaml:"tagPattern,omitempty"`        // Regex to match desired tags
	ExcludePattern    string                  `yaml:"excludePattern,omitempty"`    // Regex to exclude unwanted tags
	TagLimit          int                     `yaml:"tagLimit,omitempty"`          // Maximum number of tags to fetch from registry (before filtering)
	SortBy            string                  `yaml:"sortBy,omitempty"`            // How to sort: "semantic", "date", "alphabetical"
	Versions          []*PackageSourceVersion `yaml:"versions,omitempty"`
}

type PackageSourceVersion struct {
	Version            string `yaml:"version"`
	VersionInformation string `yaml:"versionInformation,omitempty"`
	MajorVersion       int    `yaml:"majorVersion,omitempty"`
	MinorVersion       int    `yaml:"minorVersion,omitempty"`
	PatchVersion       int    `yaml:"patchVersion,omitempty"`
}

type PackageSourceProviderType string

const (
	PackageSourceProviderTypeGitHub PackageSourceProviderType = "github"
	PackageSourceProviderTypeHarbor PackageSourceProviderType = "harbor"
	PackageSourceProviderTypeDocker PackageSourceProviderType = "docker"
)

type PackageSourceProviderAuthType string

const (
	PackageSourceProviderAuthTypeNone  PackageSourceProviderAuthType = "none"
	PackageSourceProviderAuthTypeBasic PackageSourceProviderAuthType = "basic"
	PackageSourceProviderAuthTypeToken PackageSourceProviderAuthType = "token"
)

type PackageSourceProvider struct {
	Name     string                        `yaml:"name"`
	Type     PackageSourceProviderType     `yaml:"type"`
	BaseUrl  string                        `yaml:"baseUrl,omitempty"`
	AuthType PackageSourceProviderAuthType `yaml:"authType,omitempty"`
	Username string                        `yaml:"username,omitempty"`
	Password string                        `yaml:"password,omitempty"`
	Token    string                        `yaml:"token,omitempty"`
}
