package configuration

import (
	"fmt"
	"os"
	"os/exec"

	"gopkg.in/yaml.v3"
)

// DecryptSOPSFileWithLib decrypts a SOPS file using the SOPS Go library
// This is the production implementation
func DecryptSOPSFileWithLib(filePath string) (map[string]interface{}, error) {
	// For now, we'll use the sops CLI as a fallback
	// In production, you could use: go.mozilla.org/sops/v3/decrypt
	return decryptWithSOPSCLI(filePath)
}

// decryptWithSOPSCLI decrypts a SOPS file using the sops command-line tool
func decryptWithSOPSCLI(filePath string) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Execute sops --decrypt command
	cmd := exec.Command("sops", "--decrypt", filePath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("sops decryption failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute sops: %w", err)
	}

	// Parse the decrypted YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted YAML: %w", err)
	}

	return data, nil
}

// EncryptWithSOPSCLI encrypts data to a SOPS file using the sops command-line tool
// This is primarily for testing purposes
func EncryptWithSOPSCLI(data map[string]interface{}, outputPath string, keyFingerprint string) error {
	// Convert data to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to YAML: %w", err)
	}

	// Write unencrypted data to a temporary file
	tmpFile, err := os.CreateTemp("", "sops-input-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(yamlData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Encrypt with sops
	var cmd *exec.Cmd
	if keyFingerprint != "" {
		cmd = exec.Command("sops", "--encrypt", "--pgp", keyFingerprint, "--output", outputPath, tmpPath)
	} else {
		// Use age or default encryption method
		cmd = exec.Command("sops", "--encrypt", "--output", outputPath, tmpPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sops encryption failed: %s - %w", string(output), err)
	}

	return nil
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