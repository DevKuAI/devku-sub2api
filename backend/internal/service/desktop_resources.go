package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const DesktopResourceMaxBytes = 64 * 1024 * 1024

var (
	ErrDesktopResourceNotFound = infraerrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	ErrDesktopResourceConflict = infraerrors.Conflict("RESOURCE_VERSION_CONFLICT", "resource version already exists or is older than the current version")
	ErrDesktopResourceInvalid  = infraerrors.BadRequest("RESOURCE_PACKAGE_INVALID", "invalid resource package")
	resourceKeyPattern         = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	resourceVersionPattern     = regexp.MustCompile(`^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$`)
)

type DesktopResourceManifest struct {
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
type DesktopResourceArtifact struct {
	Platform  string                  `json:"platform"`
	SHA256    string                  `json:"sha256"`
	SizeBytes int64                   `json:"sizeBytes"`
	Manifest  DesktopResourceManifest `json:"manifest"`
}
type DesktopResource struct {
	ID          string                    `json:"id"`
	Key         string                    `json:"key"`
	Kind        string                    `json:"kind"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	SourceURL   string                    `json:"sourceUrl"`
	Version     string                    `json:"version"`
	Scope       string                    `json:"scope"`
	Artifacts   []DesktopResourceArtifact `json:"artifacts"`
}
type DesktopResourceQuery struct {
	Scope, Kind, Search, ID string
	Limit, Offset           int
}
type DesktopResourceRepository interface {
	List(context.Context, int64, DesktopResourceQuery) ([]DesktopResource, error)
	GetArtifact(context.Context, int64, string, string, string) (*DesktopResourceArtifact, []byte, error)
	Publish(context.Context, *int64, *int64, DesktopResourceManifest, string, []byte) (*DesktopResource, error)
}
type DesktopResourceService struct{ repo DesktopResourceRepository }

func NewDesktopResourceService(repo DesktopResourceRepository) *DesktopResourceService {
	return &DesktopResourceService{repo: repo}
}
func (s *DesktopResourceService) List(ctx context.Context, member *DesktopAuthorizedMember, q DesktopResourceQuery) ([]DesktopResource, error) {
	if member == nil || member.Organization == nil || member.Member == nil {
		return nil, ErrDesktopUnauthenticated
	}
	if q.Scope != "public" && q.Scope != "enterprise" {
		return nil, ErrDesktopValidation
	}
	if q.Kind != "" && q.Kind != "mcp" && q.Kind != "skill" {
		return nil, ErrDesktopValidation
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}
	if q.Offset < 0 || q.Offset > 10000 || len(q.Search) > 200 {
		return nil, ErrDesktopValidation
	}
	return s.repo.List(ctx, member.Organization.ID, q)
}
func (s *DesktopResourceService) Get(ctx context.Context, member *DesktopAuthorizedMember, id string) (*DesktopResource, error) {
	if member == nil || member.Organization == nil || member.Member == nil {
		return nil, ErrDesktopUnauthenticated
	}
	if len(id) > 100 {
		return nil, ErrDesktopResourceNotFound
	}
	rows, err := s.repo.List(ctx, member.Organization.ID, DesktopResourceQuery{Scope: "visible", ID: id, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrDesktopResourceNotFound
	}
	return &rows[0], nil
}
func (s *DesktopResourceService) Download(ctx context.Context, member *DesktopAuthorizedMember, id, version, platform string) (*DesktopResourceArtifact, []byte, error) {
	if member == nil || member.Organization == nil || member.Member == nil {
		return nil, nil, ErrDesktopUnauthenticated
	}
	if !resourceVersionPattern.MatchString(version) || len(id) > 100 || len(platform) > 40 {
		return nil, nil, ErrDesktopResourceNotFound
	}
	return s.repo.GetArtifact(ctx, member.Organization.ID, id, version, platform)
}
func (s *DesktopResourceService) PublishPublic(ctx context.Context, body []byte) (*DesktopResource, error) {
	manifest, checksum, err := ValidateDesktopResourcePackage(body)
	if err != nil {
		return nil, err
	}
	return s.repo.Publish(ctx, nil, nil, *manifest, checksum, body)
}

func safeResourcePath(name string) bool {
	if name == "" || len(name) > 240 || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00") || path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsFunc(part, unicode.IsControl) {
			return false
		}
		stem := strings.ToUpper(strings.Split(part, ".")[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
			return false
		}
	}
	return true
}
func ValidateDesktopResourcePackage(body []byte) (*DesktopResourceManifest, string, error) {
	if len(body) == 0 || len(body) > DesktopResourceMaxBytes {
		return nil, "", ErrDesktopResourceInvalid
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil || len(archive.File) > 1024 {
		return nil, "", ErrDesktopResourceInvalid
	}
	files := map[string][]byte{}
	names := map[string]bool{}
	var total uint64
	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, "/")
		if names[strings.ToLower(name)] {
			return nil, "", ErrDesktopResourceInvalid
		}
		names[strings.ToLower(name)] = true
		if !safeResourcePath(name) || (!file.Mode().IsRegular() && !file.FileInfo().IsDir()) {
			return nil, "", ErrDesktopResourceInvalid
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if _, exists := files[name]; exists {
			return nil, "", ErrDesktopResourceInvalid
		}
		total += file.UncompressedSize64
		if total > 128*1024*1024 || file.UncompressedSize64 > DesktopResourceMaxBytes {
			return nil, "", ErrDesktopResourceInvalid
		}
		reader, err := file.Open()
		if err != nil {
			return nil, "", ErrDesktopResourceInvalid
		}
		content, err := io.ReadAll(io.LimitReader(reader, DesktopResourceMaxBytes+1))
		_ = reader.Close()
		if err != nil || len(content) > DesktopResourceMaxBytes {
			return nil, "", ErrDesktopResourceInvalid
		}
		files[name] = content
	}
	raw, exists := files["manifest.json"]
	if !exists || len(raw) > 128*1024 {
		return nil, "", ErrDesktopResourceInvalid
	}
	var manifest DesktopResourceManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, "", ErrDesktopResourceInvalid
	}
	if manifest.SchemaVersion != 1 || !resourceKeyPattern.MatchString(manifest.Key) || !resourceVersionPattern.MatchString(manifest.Version) || strings.TrimSpace(manifest.Name) == "" || len(manifest.Name) > 200 || len(manifest.Description) > 2000 || len(manifest.SourceURL) > 2048 || !safeResourcePath(manifest.Entry) {
		return nil, "", ErrDesktopResourceInvalid
	}
	if manifest.SourceURL != "" {
		u, err := url.Parse(manifest.SourceURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return nil, "", ErrDesktopResourceInvalid
		}
	}
	if manifest.Kind != "mcp" && manifest.Kind != "skill" {
		return nil, "", ErrDesktopResourceInvalid
	}
	if manifest.CredentialMode != "" && manifest.CredentialMode != "enterprise_model" {
		return nil, "", ErrDesktopResourceInvalid
	}
	if len(manifest.Args) > 32 || len(manifest.Targets) == 0 || len(manifest.Targets) > 2 {
		return nil, "", ErrDesktopResourceInvalid
	}
	targetSet := map[string]bool{}
	for _, target := range manifest.Targets {
		if targetSet[target] {
			return nil, "", ErrDesktopResourceInvalid
		}
		targetSet[target] = true
		if target != "workbuddy" && target != "chatgpt_codex" {
			return nil, "", ErrDesktopResourceInvalid
		}
	}
	for _, arg := range manifest.Args {
		if len(arg) > 1000 || strings.ContainsRune(arg, 0) {
			return nil, "", ErrDesktopResourceInvalid
		}
	}
	if manifest.Kind == "skill" {
		if manifest.Platform != "any" || manifest.Entry != "skill/SKILL.md" || manifest.CredentialMode != "" || len(manifest.Args) > 0 {
			return nil, "", ErrDesktopResourceInvalid
		}
	} else if manifest.Platform != "darwin-arm64" && manifest.Platform != "darwin-x64" && manifest.Platform != "windows-x64" && manifest.Platform != "linux-x64" {
		return nil, "", ErrDesktopResourceInvalid
	}
	if _, ok := files[manifest.Entry]; !ok {
		return nil, "", ErrDesktopResourceInvalid
	}
	if len(manifest.Files) != len(files)-1 {
		return nil, "", ErrDesktopResourceInvalid
	}
	for name, checksum := range manifest.Files {
		for other := range files {
			if strings.HasPrefix(strings.ToLower(other), strings.ToLower(name)+"/") {
				return nil, "", ErrDesktopResourceInvalid
			}
		}
		contents, ok := files[name]
		if !ok || name == "manifest.json" || !safeResourcePath(name) || (manifest.Kind == "skill" && !strings.HasPrefix(name, "skill/")) {
			return nil, "", ErrDesktopResourceInvalid
		}
		digest := sha256.Sum256(contents)
		if hex.EncodeToString(digest[:]) != checksum {
			return nil, "", ErrDesktopResourceInvalid
		}
	}
	checksum := sha256.Sum256(body)
	return &manifest, hex.EncodeToString(checksum[:]), nil
}
