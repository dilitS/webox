---
name: secret-flow
description: Handle any code path that touches secrets (passwords, tokens, SSH keys, AES-GCM, keyring) following Webox's strict policy. Use when adding/modifying anything in secrets/, when implementing secret-handling in providers/, services/github, when writing crypto code, or when reviewing PRs that touch authentication/credentials.
---

# Secret Flow — Webox

## Cardinal rules

1. **Secrets NEVER touch disk in plaintext outside keyring or `secrets.enc`.**
2. **Secrets NEVER enter `fmt.Sprintf`, `fmt.Errorf`, `log.*`, error messages, or stack traces.**
3. **AES-GCM nonce ONLY from `crypto/rand.Read(12 bytes)`. NEVER time-based, counter-based, or deterministic.**
4. **`config.json` must NEVER contain a secret-shaped string.** JSON Schema validator must reject `ghp_`, `ghs_`, `github_pat_`, `sk-`, `BEGIN ... PRIVATE KEY`.

## Code patterns

### Keyring set + get

```go
// secrets/keyring.go
import "github.com/zalando/go-keyring"

const service = "webox"

func SetGitHubToken(alias, token string) error {
    if err := keyring.Set(service+"-gh-token", alias, token); err != nil {
        return fmt.Errorf("keyring set gh token for %s: %w", alias, err)
    }
    return nil
}

func GetGitHubToken(alias string) (string, error) {
    token, err := keyring.Get(service+"-gh-token", alias)
    if err != nil {
        if errors.Is(err, keyring.ErrNotFound) {
            return "", ErrTokenMissing
        }
        return "", fmt.Errorf("keyring get gh token for %s: %w", alias, err)
    }
    return token, nil
}
```

**Note:** `keyring.ErrNotFound` here is **not** a fallback trigger — it just means the secret hasn't been stored yet. Webox prompts the user to enter it.

### Keyring detection (probe write/read/delete)

```go
func detectKeyringAvailable() (available bool, reason string) {
    const probeKey = "webox-probe"
    const probeUser = "sentinel"

    tokenBytes := make([]byte, 16)
    if _, err := rand.Read(tokenBytes); err != nil {
        return false, fmt.Sprintf("csprng: %v", err)
    }
    token := hex.EncodeToString(tokenBytes)

    if err := keyring.Set(probeKey, probeUser, token); err != nil {
        if errors.Is(err, keyring.ErrUnsupportedPlatform) {
            return false, "keyring unsupported on this platform"
        }
        return false, fmt.Sprintf("keyring set: %v", err)
    }

    got, err := keyring.Get(probeKey, probeUser)
    if err != nil {
        _ = keyring.Delete(probeKey, probeUser)
        return false, fmt.Sprintf("keyring get after set: %v", err)
    }
    if got != token {
        _ = keyring.Delete(probeKey, probeUser)
        return false, "keyring returned different value"
    }

    if err := keyring.Delete(probeKey, probeUser); err != nil {
        return false, fmt.Sprintf("keyring delete probe: %v", err)
    }
    return true, ""
}
```

### AES-GCM encrypt entry

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
)

func encryptEntry(masterKey []byte, plaintext []byte) (nonce, ciphertext []byte, err error) {
    block, err := aes.NewCipher(masterKey)
    if err != nil {
        return nil, nil, fmt.Errorf("aes new cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, nil, fmt.Errorf("cipher new gcm: %w", err)
    }
    nonce = make([]byte, gcm.NonceSize()) // 12 bytes for GCM
    if _, err := rand.Read(nonce); err != nil {
        return nil, nil, fmt.Errorf("csprng failure for nonce: %w", err)
    }
    ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
    return nonce, ciphertext, nil
}
```

**Test that two consecutive calls produce different nonces:**

```go
func TestEncryptEntry_NonceUniqueness(t *testing.T) {
    key := bytes.Repeat([]byte{0x01}, 32)
    plaintext := []byte("same plaintext")

    nonce1, _, err := encryptEntry(key, plaintext)
    if err != nil { t.Fatal(err) }
    nonce2, _, err := encryptEntry(key, plaintext)
    if err != nil { t.Fatal(err) }

    if bytes.Equal(nonce1, nonce2) {
        t.Fatalf("nonces must differ: nonce1=%x nonce2=%x", nonce1, nonce2)
    }
}
```

### Argon2id KDF

```go
import "golang.org/x/crypto/argon2"

const (
    argonMemory      uint32 = 64 * 1024 // 64 MB
    argonIterations  uint32 = 3
    argonParallelism uint8  = 2
    argonKeyLen      uint32 = 32 // 256-bit key
    argonSaltLen     int    = 16
)

func deriveKey(password []byte, salt []byte) []byte {
    return argon2.IDKey(password, salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)
}
```

### Sensitive variable in memory

```go
import "github.com/awnumar/memguard"

func operationNeedingPassword(masterPasswordBytes []byte) error {
    locked := memguard.NewBufferFromBytes(masterPasswordBytes)
    defer locked.Destroy()

    key := deriveKey(locked.Bytes(), salt)
    // use key, then ...
    memguard.WipeBytes(key)
    return nil
}
```

### Logging — redactor

```go
// internal/log/redact.go
var redactPatterns = []*regexp.Regexp{
    regexp.MustCompile(`gh[ps]_[A-Za-z0-9]{36,255}`),
    regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`),
    regexp.MustCompile(`ey[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
    regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`),
    regexp.MustCompile(`(?i)(password|passwd|secret|token)\s*[:=]\s*\S+`),
}

func Redact(s string) string {
    for _, p := range redactPatterns {
        s = p.ReplaceAllString(s, "*** REDACTED ***")
    }
    return s
}
```

### `WEBOX_MASTER_PASSWORD` warning

```go
func warnIfMasterPasswordOnWorkstation() {
    if os.Getenv("WEBOX_MASTER_PASSWORD") == "" {
        return
    }
    isCI := os.Getenv("CI") == "true" ||
            os.Getenv("GITHUB_ACTIONS") == "true" ||
            os.Getenv("GITLAB_CI") == "true"
    isWorkstation := os.Getenv("SSH_CLIENT") != "" ||
                     os.Getenv("DISPLAY") != "" ||
                     os.Getenv("XDG_SESSION_TYPE") != ""
    if !isCI && isWorkstation {
        log.Warn("WEBOX_MASTER_PASSWORD is set on a workstation. " +
                 "This env var is readable via /proc/<pid>/environ, ps eaux, etc. " +
                 "Use interactive unlock instead, or set CI=true if this is intentional CI run.")
    }
}
```

## Anti-patterns (instant reject)

```go
// ❌ Secret in error
return fmt.Errorf("auth failed for %s with password %s", user, password)

// ❌ Secret in log
log.Info("connecting", "user", user, "token", token)

// ❌ Time-based nonce
nonce := make([]byte, 12)
binary.BigEndian.PutUint64(nonce, uint64(time.Now().UnixNano()))

// ❌ Counter nonce
nonceCounter++
nonce := make([]byte, 12)
binary.BigEndian.PutUint64(nonce, nonceCounter)

// ❌ Secret in stack trace via Sprintf
panic(fmt.Sprintf("decryption failed for entry %s: %v", key, password))

// ❌ Secret in cassette / fixture
# .yaml cassette captures real token: Authorization: Bearer ghp_realToken123...

// ❌ Secret in config.json
{"profiles": [{"alias": "main", "password": "actual-password"}]}

// ❌ Clipboard "best-effort" cleanup with timer
go func() {
    time.Sleep(30 * time.Second)
    clipboard.WriteAll("")  // false promise; clipboard managers retain history
}()
```

## Checklist before commit

```
- [ ] No fmt.Sprintf / fmt.Errorf with secret value.
- [ ] No log.* call with secret variable.
- [ ] No secret in test fixture (sanitize cassettes).
- [ ] AES-GCM nonce from crypto/rand.Read.
- [ ] Test verifying nonce uniqueness present.
- [ ] memguard.LockedBuffer for in-flight secret bytes.
- [ ] Redactor patterns updated if introducing new secret format.
- [ ] CHANGELOG entry under [Unreleased] / Security if behavior changes.
- [ ] No false promises (e.g. clipboard auto-clear).
```

## References

- [SECURITY.md §3.2](../../docs/SECURITY.md#32-macierz-zagro%C5%BCe%C5%84) — threat matrix.
- [SECURITY.md §4.2](../../docs/SECURITY.md#42-fallback-dla-środowisk-headless) — keyring + fallback.
- [SECURITY.md §4.2.1](../../docs/SECURITY.md#421-generowanie-nonce-krytyczne-dla-aes-gcm) — AES-GCM nonce rules.
- [SECURITY.md §9.2](../../docs/SECURITY.md#92-redaktor-sekret%C3%B3w) — redactor patterns.
- [AUDIT §8 IMP-2, IMP-3, IMP-9](../../docs/AUDIT.md#8-uzupe%C5%82niaj%C4%85ce-znaleziska-po-drugim-przebiegu) — secret-related findings.
