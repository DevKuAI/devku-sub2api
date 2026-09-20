// Package desktopresource validates portable MCP and Skill archives without executing them.
package desktopresource

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxZIPBytes      int64 = 64 << 20
	MaxExpandedBytes int64 = 128 << 20
	MaxManifestBytes int64 = 128 << 10
	MaxEntries             = 1024
)

var (
	KeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	VersionPattern  = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$`)
	hashPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	windowsReserved = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])$`)
)

type Manifest struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Key            string            `json:"key"`
	Kind           string            `json:"kind"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	SourceURL      string            `json:"sourceUrl"`
	Version        string            `json:"version"`
	Platform       string            `json:"platform"`
	Entry          string            `json:"entry"`
	Args           []string          `json:"args,omitempty"`
	CredentialMode string            `json:"credentialMode,omitempty"`
	Targets        []string          `json:"targets"`
	Files          map[string]string `json:"files"`
}

type Package struct {
	Manifest  Manifest `json:"manifest"`
	SHA256    string   `json:"sha256"`
	SizeBytes int64    `json:"sizeBytes"`
}

func ValidPlatform(value string, skill bool) bool {
	if skill {
		return value == "any"
	}
	switch value {
	case "darwin-arm64", "darwin-x64", "windows-x64", "linux-x64":
		return true
	}
	return false
}

func safePath(value string) bool {
	if value == "" || len(value) > 240 || !utf8.ValidString(value) || strings.ContainsAny(value, "\\:<>\"|?*\x00") || strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." || strings.TrimRight(part, ". ") != part || windowsReserved.MatchString(strings.SplitN(part, ".", 2)[0]) {
			return false
		}
		for _, r := range part {
			if r < 32 {
				return false
			}
		}
	}
	return true
}

func (m *Manifest) Validate() error {
	if m.SchemaVersion != 1 || !KeyPattern.MatchString(m.Key) || !VersionPattern.MatchString(m.Version) || (m.Kind != "mcp" && m.Kind != "skill") {
		return fmt.Errorf("invalid schemaVersion, key, version or kind")
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 200 || len(m.Description) > 2000 || len(m.SourceURL) > 2048 {
		return fmt.Errorf("invalid resource metadata length")
	}
	if m.SourceURL != "" {
		u, err := url.Parse(m.SourceURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("sourceUrl must be an HTTPS URL without userinfo")
		}
	}
	if !ValidPlatform(m.Platform, m.Kind == "skill") || !safePath(m.Entry) || len(m.Files) == 0 || len(m.Files) >= MaxEntries {
		return fmt.Errorf("invalid platform, entry or files")
	}
	if _, ok := m.Files[m.Entry]; !ok {
		return fmt.Errorf("entry must be declared in files")
	}
	if len(m.Args) > 32 || (m.CredentialMode != "" && m.CredentialMode != "enterprise_model") {
		return fmt.Errorf("invalid args or credentialMode")
	}
	for _, arg := range m.Args {
		if len(arg) > 1000 || strings.ContainsRune(arg, 0) {
			return fmt.Errorf("invalid argument")
		}
	}
	if len(m.Targets) < 1 || len(m.Targets) > 2 {
		return fmt.Errorf("invalid targets")
	}
	seen := map[string]bool{}
	for _, target := range m.Targets {
		if seen[target] || (target != "workbuddy" && target != "chatgpt_codex") {
			return fmt.Errorf("invalid or duplicate target")
		}
		seen[target] = true
	}
	for name, hash := range m.Files {
		if !safePath(name) || strings.EqualFold(name, "manifest.json") || !hashPattern.MatchString(hash) {
			return fmt.Errorf("invalid files entry: %s", name)
		}
		if m.Kind == "skill" && !strings.HasPrefix(name, "skill/") {
			return fmt.Errorf("skill files must be below skill/")
		}
	}
	if m.Kind == "skill" && (m.Entry != "skill/SKILL.md" || len(m.Args) != 0 || m.CredentialMode != "") {
		return fmt.Errorf("invalid Skill installation fields")
	}
	return nil
}

// Reject duplicate object keys as well as unknown fields to keep Go and Rust interpretations identical.
func uniqueJSON(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for dec.More() {
		if delim == '{' {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate JSON key")
			}
			seen[name] = true
		}
		if err := uniqueJSON(dec); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}

func parseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if !utf8.Valid(raw) {
		return m, fmt.Errorf("manifest is not UTF-8")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return m, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return m, err
	}
	for _, key := range []string{"schemaVersion", "key", "kind", "name", "description", "sourceUrl", "version", "platform", "entry", "targets", "files"} {
		if value, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return m, fmt.Errorf("missing field: %s", key)
		}
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return m, fmt.Errorf("null manifest fields are not supported")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	return m, m.Validate()
}

func archiveEntries(archive *zip.Reader) (map[string]*zip.File, error) {
	if len(archive.File) == 0 || len(archive.File) > MaxEntries {
		return nil, fmt.Errorf("invalid ZIP entry count")
	}
	files := map[string]*zip.File{}
	paths := map[string]string{}
	var total uint64
	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, "/")
		mode := file.Mode()
		if !safePath(name) || (!mode.IsRegular() && !mode.IsDir()) || file.Flags&1 != 0 {
			return nil, fmt.Errorf("unsafe ZIP entry: %s", file.Name)
		}
		folded := strings.ToLower(name)
		if _, ok := paths[folded]; ok {
			return nil, fmt.Errorf("duplicate ZIP path: %s", name)
		}
		paths[folded] = name
		if file.UncompressedSize64 > uint64(MaxZIPBytes) {
			return nil, fmt.Errorf("ZIP entry too large")
		}
		total += file.UncompressedSize64
		if total > uint64(MaxExpandedBytes) {
			return nil, fmt.Errorf("ZIP expands beyond limit")
		}
		if !mode.IsDir() {
			files[name] = file
		}
	}
	// Reject file/directory collisions, including implicitly created parent directories.
	parents := map[string]string{}
	for name := range paths {
		original := paths[name]
		for parent := path.Dir(original); parent != "."; parent = path.Dir(parent) {
			folded := strings.ToLower(parent)
			if prior, ok := parents[folded]; ok && prior != parent {
				return nil, fmt.Errorf("case-colliding directories")
			}
			parents[folded] = parent
			if prior, ok := paths[folded]; ok {
				if prior != parent || files[prior] != nil {
					return nil, fmt.Errorf("conflicting ZIP directory")
				}
			}
		}
	}
	return files, nil
}

func Inspect(filename string) (*Package, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > MaxZIPBytes {
		return nil, fmt.Errorf("invalid ZIP size")
	}
	archive, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, err
	}
	files, err := archiveEntries(archive)
	if err != nil {
		return nil, err
	}
	manifest := files["manifest.json"]
	if manifest == nil || manifest.UncompressedSize64 > uint64(MaxManifestBytes) {
		return nil, fmt.Errorf("missing or oversized manifest.json")
	}
	r, err := manifest.Open()
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxManifestBytes+1))
	_ = r.Close()
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > MaxManifestBytes {
		return nil, fmt.Errorf("manifest exceeds limit")
	}
	m, err := parseManifest(raw)
	if err != nil {
		return nil, err
	}
	if len(files) != len(m.Files)+1 {
		return nil, fmt.Errorf("ZIP files do not match manifest")
	}
	total := int64(len(raw))
	for name, expected := range m.Files {
		file := files[name]
		if file == nil {
			return nil, fmt.Errorf("missing declared file: %s", name)
		}
		r, err := file.Open()
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(r, MaxZIPBytes+1))
		_ = r.Close()
		total += n
		if err != nil {
			return nil, err
		}
		if n > MaxZIPBytes || total > MaxExpandedBytes || hex.EncodeToString(h.Sum(nil)) != expected {
			return nil, fmt.Errorf("file size or hash mismatch: %s", name)
		}
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return &Package{Manifest: m, SHA256: hex.EncodeToString(h.Sum(nil)), SizeBytes: info.Size()}, nil
}
