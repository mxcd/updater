# Variable Substitution

The updater configuration supports variable substitution for sensitive data and environment-specific values. This allows you to:

1. Keep secrets out of configuration files
2. Use the same configuration across different environments
3. Centralize secret management using SOPS

## Supported Substitution Types

### 1. Environment Variables

Use `${VAR_NAME}` syntax to reference environment variables:

```yaml
packageSourceProviders:
  - name: github-enterprise
    type: github
    baseUrl: ${GITHUB_BASE_URL}
    authType: token
    token: ${GITHUB_TOKEN}
```

Environment variables should be loaded before running the updater. You can use a `.env` file with `godotenv`:

```bash
# .env
GITHUB_BASE_URL=https://github.enterprise.com
GITHUB_TOKEN=ghp_your_token_here
```

### 2. SOPS Encrypted Files

Use `${SOPS[path/to/file.yml].yaml.path.to.value}` syntax to reference values in SOPS-encrypted files:

```yaml
packageSourceProviders:
  - name: git.i.mercedes-benz.com
    type: github
    baseUrl: ${BASE_URL}
    authType: token
    token: ${SOPS[sandbox/credentials.secret.enc.yml].token}
```

## SOPS Integration

The updater uses the official SOPS Go library (`github.com/getsops/sops/v3`) for decrypting encrypted configuration files. This provides native integration with all SOPS features.

### What is SOPS?

[SOPS](https://github.com/getsops/sops) (Secrets OPerationS) is an encryption tool that encrypts YAML, JSON, and other file formats while keeping the structure readable. It supports multiple encryption backends:

- Age
- PGP/GPG
- AWS KMS
- GCP KMS
- Azure Key Vault
- HashiCorp Vault

### Setting Up SOPS

1. **Install SOPS** (optional for CLI usage, library is built-in):
   ```bash
   # macOS
   brew install sops
   
   # Linux
   # Download from https://github.com/getsops/sops/releases
   ```

2. **Create an encryption key**:
   
   Using Age (recommended for simplicity):
   ```bash
   # Install age
   brew install age  # or apt-get install age
   
   # Generate a key
   age-keygen -o ~/.config/sops/age/keys.txt
   ```
   
   Using GPG:
   ```bash
   gpg --gen-key
   gpg --list-keys  # Note the key fingerprint
   ```

3. **Configure SOPS**:
   
   Create `.sops.yaml` in your project root:
   ```yaml
   creation_rules:
     - path_regex: \.secret\.(enc\.)?ya?ml$
       age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
   ```
   
   Or for GPG:
   ```yaml
   creation_rules:
     - path_regex: \.secret\.(enc\.)?ya?ml$
       pgp: 'YOUR_GPG_KEY_FINGERPRINT'
   ```

4. **Create and encrypt a secrets file**:
   ```bash
   # Create your secrets file
   cat > sandbox/credentials.secret.enc.yml <<EOF
   token: ghp_your_actual_token_here
   username: your_username
   password: your_password
   EOF
   
   # Encrypt it
   sops --encrypt --in-place sandbox/credentials.secret.enc.yml
   ```

5. **Verify encryption**:
   ```bash
   # View the encrypted file
   cat sandbox/credentials.secret.enc.yml
   
   # Decrypt and view (requires your key)
   sops --decrypt sandbox/credentials.secret.enc.yml
   ```

### SOPS YAML Path Access

The SOPS substitution supports dot notation to access nested values:

```yaml
# credentials.secret.enc.yml (decrypted)
github:
  token: ghp_token123
  webhook_secret: secret456
docker:
  username: dockeruser
  password: dockerpass

api:
  keys:
    prod: prod-key-789
    dev: dev-key-012
```

Reference these values in your config:

```yaml
packageSourceProviders:
  - name: github
    token: ${SOPS[credentials.secret.enc.yml].github.token}
  
  - name: docker
    username: ${SOPS[credentials.secret.enc.yml].docker.username}
    password: ${SOPS[credentials.secret.enc.yml].docker.password}
  
  - name: api-prod
    token: ${SOPS[credentials.secret.enc.yml].api.keys.prod}
```

## File Caching

SOPS files are cached after first decryption to improve performance. Each unique file is decrypted only once per configuration load, even if referenced multiple times.

## Security Best Practices

1. **Never commit unencrypted secrets**
   - Always encrypt with SOPS before committing
   - Use `.gitignore` to exclude `.env` and unencrypted secret files

2. **Restrict file permissions**:
   ```bash
   chmod 600 sandbox/credentials.secret.enc.yml
   chmod 600 ~/.config/sops/age/keys.txt
   ```

3. **Use different keys for different environments**:
   ```yaml
   # .sops.yaml
   creation_rules:
     - path_regex: prod/.*\.secret\.ya?ml$
       age: age1prod_key_here
     - path_regex: dev/.*\.secret\.ya?ml$
       age: age1dev_key_here
   ```

4. **Rotate keys periodically**:
   ```bash
   # Re-encrypt with new key
   sops rotate --in-place credentials.secret.enc.yml
   ```

## Error Handling

The updater will fail to start if:
- A referenced environment variable is not set
- A SOPS file cannot be decrypted (missing key)
- A YAML path doesn't exist in the SOPS file
- SOPS is not installed (when using SOPS references)

Error messages will clearly indicate which substitution failed:

```
Error: failed to substitute variables: failed to substitute Token in provider git.i.mercedes-benz.com: 
failed to resolve SOPS reference ${SOPS[credentials.secret.enc.yml].token}: 
failed to decrypt SOPS file: sops decryption failed: no decryption key found
```

## Example Configuration

See [`sandbox/config.yml`](../sandbox/config.yml) and [`sandbox/.env.example`](../sandbox/.env.example) for complete examples.

## Testing

Run the test suite to verify substitution functionality:

```bash
# Run all configuration tests
go test ./internal/configuration/...

# Run only substitution tests
go test ./internal/configuration/ -run TestSubstitute

# Run SOPS E2E tests
go test ./internal/configuration/ -run TestSOPS