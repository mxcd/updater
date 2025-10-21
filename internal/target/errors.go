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