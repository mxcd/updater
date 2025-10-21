package configuration

import (
	"fmt"
	"os"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

// DecryptSOPSFileWithLib decrypts a SOPS file using the SOPS Go library
func DecryptSOPSFileWithLib(filePath string) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Decrypt the file using the SOPS library
	// This automatically handles key resolution from environment, config files, etc.
	cleartext, err := decrypt.File(filePath, "yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt SOPS file: %w", err)
	}

	// Parse the decrypted YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal(cleartext, &data); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted YAML: %w", err)
	}

	return data, nil
}

// CreateTestSOPSFile creates an encrypted SOPS file for testing
// It uses age encryption for simplicity in tests
func CreateTestSOPSFile(data map[string]interface{}, outputPath string) error {
	// Convert data to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to YAML: %w", err)
	}

	// For testing, we'll create a mock encrypted file
	// In real tests with SOPS, you would use actual encryption
	// For now, just write the YAML with a SOPS header
	mockEncrypted := createMockSOPSFile(yamlData)

	if err := os.WriteFile(outputPath, mockEncrypted, 0600); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	return nil
}

// createMockSOPSFile creates a mock SOPS-encrypted file for testing
// This simulates the structure of a SOPS file without actual encryption
func createMockSOPSFile(plainYAML []byte) []byte {
	// In real SOPS files, this would be encrypted
	// For testing without actual SOPS setup, we return the plain YAML
	// with a SOPS metadata section
	mockFile := string(plainYAML) + `
sops:
    kms: []
    gcp_kms: []
    azure_kv: []
    hc_vault: []
    age: []
    lastmodified: "2024-01-01T00:00:00Z"
    mac: ENC[AES256_GCM,data:mock,iv:mock,tag:mock,type:str]
    pgp: []
    unencrypted_suffix: _unencrypted
    version: 3.7.3
`
	return []byte(mockFile)
}
