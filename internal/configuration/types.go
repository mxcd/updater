package configuration

type Config struct {
	PackageSourceProviders []*PackageSourceProvider `yaml:"packageSourceProviders"`
	PackageSources         []*PackageSource         `yaml:"packageSources"`
	Targets                []*Target                `yaml:"targets"`
	TargetActor            *TargetActor             `yaml:"targetActor,omitempty"`
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
	Branch            string                  `yaml:"branch,omitempty"` // Git branch (for git-helm-chart), defaults to "main"
	Path              string                  `yaml:"path,omitempty"`   // File path in repository (for git-helm-chart)
	VersionConstraint string                  `yaml:"versionConstraint,omitempty"`
	TagPattern        string                  `yaml:"tagPattern,omitempty"`     // Regex to match desired tags
	ExcludePattern    string                  `yaml:"excludePattern,omitempty"` // Regex to exclude unwanted tags
	TagLimit          int                     `yaml:"tagLimit,omitempty"`       // Maximum number of tags to fetch from registry (before filtering)
	SortBy            string                  `yaml:"sortBy,omitempty"`         // How to sort: "semantic", "date", "alphabetical"
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

type TargetType string

const (
	TargetTypeTerraformVariable TargetType = "terraform-variable"
)

type Target struct {
	Name       string       `yaml:"name"`
	Type       TargetType   `yaml:"type"`
	File       string       `yaml:"file"`
	Items      []TargetItem `yaml:"items"`
	PatchGroup string       `yaml:"patchGroup,omitempty"`
	Labels     []string     `yaml:"labels,omitempty"`
}

type TargetItem struct {
	Name                  string   `yaml:"name,omitempty"`
	TerraformVariableName string   `yaml:"terraformVariableName,omitempty"`
	Source                string   `yaml:"source"`
	PatchGroup            string   `yaml:"patchGroup,omitempty"`
	Labels                []string `yaml:"labels,omitempty"`
}

type TargetActor struct {
	Name     string `yaml:"name"`
	Email    string `yaml:"email"`
	Username string `yaml:"username"`
	Token    string `yaml:"token,omitempty"`
}
