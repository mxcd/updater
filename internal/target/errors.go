package target

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
)

// UnsupportedTargetTypeError is returned when an unsupported target type is encountered
type UnsupportedTargetTypeError struct {
	Type configuration.TargetType
}

func (e *UnsupportedTargetTypeError) Error() string {
	return fmt.Sprintf("unsupported target type: %s", e.Type)
}

// FileNotFoundError is returned when a target file is not found
type FileNotFoundError struct {
	Path string
}

func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("target file not found: %s", e.Path)
}

// VariableNotFoundError is returned when a variable is not found in the target file
type VariableNotFoundError struct {
	Variable string
	File     string
}

func (e *VariableNotFoundError) Error() string {
	return fmt.Sprintf("variable '%s' not found in file: %s", e.Variable, e.File)
}

// InvalidFileFormatError is returned when a target file has an invalid format
type InvalidFileFormatError struct {
	File   string
	Reason string
}

func (e *InvalidFileFormatError) Error() string {
	return fmt.Sprintf("invalid file format '%s': %s", e.File, e.Reason)
}

// DependencyNotFoundError is returned when a dependency is not found in the Chart.yaml file
type DependencyNotFoundError struct {
	Dependency string
	File       string
}

func (e *DependencyNotFoundError) Error() string {
	return fmt.Sprintf("dependency '%s' not found in file: %s", e.Dependency, e.File)
}

// YamlFieldNotFoundError is returned when a YAML path cannot be resolved in the target file
type YamlFieldNotFoundError struct {
	Path string
	File string
}

func (e *YamlFieldNotFoundError) Error() string {
	return fmt.Sprintf("yaml path '%s' not found in file: %s", e.Path, e.File)
}
