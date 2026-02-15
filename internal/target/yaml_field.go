package target

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// YamlFieldTarget implements the TargetClient interface for arbitrary YAML files
type YamlFieldTarget struct {
	config       *configuration.Target
	updateItem   *configuration.TargetItem
	fileContents string
	rootNodes    []*yaml.Node // supports multi-document YAML
}

// NewYamlFieldTargetForUpdateItem creates a new yaml-field target for a specific update item
func NewYamlFieldTargetForUpdateItem(config *configuration.Target, updateItem *configuration.TargetItem) (*YamlFieldTarget, error) {
	if updateItem.YamlPath == "" {
		return nil, fmt.Errorf("yamlPath is required for yaml-field target")
	}

	target := &YamlFieldTarget{
		config:     config,
		updateItem: updateItem,
	}

	if err := target.readFile(); err != nil {
		return nil, err
	}

	return target, nil
}

// readFile reads and parses the YAML file into Node trees (supports multi-document YAML)
func (t *YamlFieldTarget) readFile() error {
	content, err := os.ReadFile(t.config.File)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileNotFoundError{Path: t.config.File}
		}
		return fmt.Errorf("failed to read file %s: %w", t.config.File, err)
	}
	t.fileContents = string(content)

	t.rootNodes = nil
	decoder := yaml.NewDecoder(strings.NewReader(t.fileContents))
	for {
		node := &yaml.Node{}
		err := decoder.Decode(node)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to parse YAML file %s: %w", t.config.File, err)
		}
		t.rootNodes = append(t.rootNodes, node)
	}

	if len(t.rootNodes) == 0 {
		return fmt.Errorf("no YAML documents found in file %s", t.config.File)
	}

	return nil
}

// findNodeInDocuments searches all documents for the given path
func (t *YamlFieldTarget) findNodeInDocuments(segments []string) (*yaml.Node, error) {
	var lastErr error
	for _, root := range t.rootNodes {
		node, err := findNode(root, segments)
		if err == nil {
			return node, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// parsePath splits a dot-notation YAML path into segments
func parsePath(path string) []string {
	return strings.Split(path, ".")
}

// findNode walks the yaml.Node tree following the given path segments
// and returns the scalar node at the end of the path
func findNode(node *yaml.Node, segments []string) (*yaml.Node, error) {
	// The root node from yaml.Unmarshal is a DocumentNode wrapping the actual content
	current := node
	if current.Kind == yaml.DocumentNode {
		if len(current.Content) == 0 {
			return nil, fmt.Errorf("empty document")
		}
		current = current.Content[0]
	}

	for _, segment := range segments {
		switch current.Kind {
		case yaml.MappingNode:
			found := false
			// MappingNode Content is key-value pairs: [key0, val0, key1, val1, ...]
			for i := 0; i < len(current.Content)-1; i += 2 {
				keyNode := current.Content[i]
				valNode := current.Content[i+1]
				if keyNode.Value == segment {
					current = valNode
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("key '%s' not found", segment)
			}

		case yaml.SequenceNode:
			idx, err := strconv.Atoi(segment)
			if err != nil {
				return nil, fmt.Errorf("expected numeric index for sequence, got '%s'", segment)
			}
			if idx < 0 || idx >= len(current.Content) {
				return nil, fmt.Errorf("index %d out of range (length %d)", idx, len(current.Content))
			}
			current = current.Content[idx]

		case yaml.AliasNode:
			// Resolve the alias and continue
			current = current.Alias
			// Re-process this segment with the resolved node
			resolved, err := findNode(current, []string{segment})
			if err != nil {
				return nil, err
			}
			current = resolved

		default:
			return nil, fmt.Errorf("cannot navigate into %v node at segment '%s'", current.Kind, segment)
		}
	}

	return current, nil
}

// isDockerImageReference checks if a value looks like a Docker image reference (image:tag)
func isDockerImageReference(value string) bool {
	lastColon := strings.LastIndex(value, ":")
	if lastColon <= 0 {
		return false
	}
	if strings.Contains(value, "://") {
		return false
	}
	tag := value[lastColon+1:]
	if strings.Contains(tag, "/") || strings.Contains(tag, " ") || tag == "" {
		return false
	}
	return true
}

// extractTagFromImageReference extracts just the tag from a Docker image reference
func extractTagFromImageReference(value string) string {
	lastColon := strings.LastIndex(value, ":")
	if lastColon <= 0 {
		return value
	}
	return value[lastColon+1:]
}

// replaceTagInImageReference replaces the tag in a Docker image reference
func replaceTagInImageReference(value, newTag string) string {
	lastColon := strings.LastIndex(value, ":")
	if lastColon <= 0 {
		return newTag
	}
	return value[:lastColon+1] + newTag
}

// ReadCurrentVersion reads the current version from the specified YAML path
func (t *YamlFieldTarget) ReadCurrentVersion() (string, error) {
	log.Debug().
		Str("file", t.config.File).
		Str("yamlPath", t.updateItem.YamlPath).
		Msg("Reading current version from YAML file")

	segments := parsePath(t.updateItem.YamlPath)
	node, err := t.findNodeInDocuments(segments)
	if err != nil {
		return "", &YamlFieldNotFoundError{
			Path: t.updateItem.YamlPath,
			File: t.config.File,
		}
	}

	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("yaml path '%s' in file %s points to a non-scalar node", t.updateItem.YamlPath, t.config.File)
	}

	value := node.Value
	// If the value is a Docker image reference (e.g., "nginx:1.25.0"),
	// extract just the tag portion for version comparison
	if isDockerImageReference(value) {
		value = extractTagFromImageReference(value)
	}

	log.Debug().
		Str("file", t.config.File).
		Str("yamlPath", t.updateItem.YamlPath).
		Str("version", value).
		Msg("Found current version")

	return value, nil
}

// WriteVersion writes a new version to the specified YAML path
func (t *YamlFieldTarget) WriteVersion(version string) error {
	log.Debug().
		Str("file", t.config.File).
		Str("yamlPath", t.updateItem.YamlPath).
		Str("version", version).
		Msg("Writing new version to YAML file")

	segments := parsePath(t.updateItem.YamlPath)
	node, err := t.findNodeInDocuments(segments)
	if err != nil {
		return &YamlFieldNotFoundError{
			Path: t.updateItem.YamlPath,
			File: t.config.File,
		}
	}

	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("yaml path '%s' in file %s points to a non-scalar node", t.updateItem.YamlPath, t.config.File)
	}

	oldValue := node.Value

	// If the current value is a Docker image reference, only replace the tag portion
	var newValue string
	if isDockerImageReference(oldValue) {
		newValue = replaceTagInImageReference(oldValue, version)
	} else {
		newValue = version
	}

	// Split file into lines for surgical replacement
	lines := strings.Split(t.fileContents, "\n")
	// yaml.Node uses 1-based line numbers
	lineIdx := node.Line - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		return fmt.Errorf("yaml node line %d out of range for file %s", node.Line, t.config.File)
	}

	line := lines[lineIdx]

	// Build the search and replacement strings based on quoting style
	var searchStr, replaceStr string
	switch node.Style {
	case yaml.DoubleQuotedStyle:
		searchStr = `"` + oldValue + `"`
		replaceStr = `"` + newValue + `"`
	case yaml.SingleQuotedStyle:
		searchStr = `'` + oldValue + `'`
		replaceStr = `'` + newValue + `'`
	default:
		// Plain, literal, folded, or flow style
		searchStr = oldValue
		replaceStr = newValue
	}

	// Use the column info to target the exact position on the line
	// yaml.Node Column is 1-based
	colIdx := node.Column - 1
	if colIdx < 0 {
		colIdx = 0
	}

	// For quoted styles, the column points to the opening quote
	// For plain styles, the column points to the start of the value
	var newLine string
	if colIdx < len(line) {
		// Search from the column position onward to avoid replacing wrong occurrences
		prefix := line[:colIdx]
		suffix := line[colIdx:]
		newSuffix := strings.Replace(suffix, searchStr, replaceStr, 1)
		if newSuffix == suffix {
			// Fallback: try replacing anywhere on the line
			newLine = strings.Replace(line, searchStr, replaceStr, 1)
		} else {
			newLine = prefix + newSuffix
		}
	} else {
		newLine = strings.Replace(line, searchStr, replaceStr, 1)
	}

	lines[lineIdx] = newLine
	newContents := strings.Join(lines, "\n")

	// Write the file
	if err := os.WriteFile(t.config.File, []byte(newContents), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", t.config.File, err)
	}

	// Update internal state
	t.fileContents = newContents

	// Re-parse the YAML to update the node trees
	if err := t.reparseNodes(); err != nil {
		return fmt.Errorf("failed to re-parse YAML file %s after write: %w", t.config.File, err)
	}

	log.Debug().
		Str("file", t.config.File).
		Str("yamlPath", t.updateItem.YamlPath).
		Str("version", version).
		Msg("Successfully wrote new version")

	return nil
}

// reparseNodes re-parses the file contents into YAML node trees
func (t *YamlFieldTarget) reparseNodes() error {
	t.rootNodes = nil
	decoder := yaml.NewDecoder(strings.NewReader(t.fileContents))
	for {
		node := &yaml.Node{}
		err := decoder.Decode(node)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		t.rootNodes = append(t.rootNodes, node)
	}
	return nil
}

// GetTargetInfo returns metadata about this target
func (t *YamlFieldTarget) GetTargetInfo() *TargetInfo {
	currentVersion, err := t.ReadCurrentVersion()
	if err != nil {
		log.Warn().Err(err).Str("file", t.config.File).Str("yamlPath", t.updateItem.YamlPath).Msg("Failed to read current version for target info")
	}
	targetName := t.updateItem.Name
	if targetName == "" {
		targetName = t.config.Name
	}
	return &TargetInfo{
		Name:         targetName,
		Type:         t.config.Type,
		File:         t.config.File,
		Source:       t.updateItem.Source,
		CurrentValue: currentVersion,
	}
}

// Validate checks if the target is valid and accessible
func (t *YamlFieldTarget) Validate() error {
	// Check if file exists and is readable
	if err := t.readFile(); err != nil {
		return err
	}

	// Check if file has .yaml or .yml extension
	fileName := strings.ToLower(t.config.File)
	if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
		return &InvalidFileFormatError{
			File:   t.config.File,
			Reason: "file must have .yaml or .yml extension",
		}
	}

	// Note: We don't check if the YAML path exists here because:
	// - When using wildcards, not all matched files may contain the path
	// - This is permissive behavior - only error if NO files match
	// - ReadCurrentVersion() and WriteVersion() will handle missing paths gracefully

	log.Debug().
		Str("file", t.config.File).
		Str("yamlPath", t.updateItem.YamlPath).
		Msg("YAML field target validation successful")

	return nil
}
