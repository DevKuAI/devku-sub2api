package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"golang.org/x/mod/semver"
)

const (
	DesktopUpdateStatusDraft     = "draft"
	DesktopUpdateStatusPublished = "published"
	DesktopUpdateStatusWithdrawn = "withdrawn"

	DesktopUpdateDarwinARM64  = "darwin-aarch64"
	DesktopUpdateDarwinX64    = "darwin-x86_64"
	DesktopUpdateWindowsX64   = "windows-x86_64"
	desktopUpdateMaxNotes     = 20000
	desktopUpdateMaxURL       = 2048
	desktopUpdateMaxSignature = 8192
	desktopUpdateMaxObjectKey = 1024
	desktopUpdateMaxFileName  = 255
	desktopUpdateSHA256Length = 64
)

var (
	ErrDesktopUpdateNotFound       = infraerrors.NotFound("DESKTOP_UPDATE_NOT_FOUND", "desktop update release not found")
	ErrDesktopUpdateVersionExists  = infraerrors.Conflict("DESKTOP_UPDATE_VERSION_EXISTS", "desktop update version already exists")
	ErrDesktopUpdateInvalidVersion = infraerrors.BadRequest("DESKTOP_UPDATE_VERSION_INVALID", "desktop update version must use MAJOR.MINOR.PATCH")
	ErrDesktopUpdateInvalidFields  = infraerrors.BadRequest("DESKTOP_UPDATE_FIELDS_INVALID", "desktop update fields are invalid")
	ErrDesktopUpdateInvalidState   = infraerrors.Conflict("DESKTOP_UPDATE_STATE_INVALID", "desktop update release state does not allow this operation")
	ErrDesktopUpdateNotNewer       = infraerrors.Conflict("DESKTOP_UPDATE_VERSION_NOT_NEWER", "desktop update version must be newer than every published release")
	ErrDesktopUpdateUnsupported    = infraerrors.BadRequest("DESKTOP_UPDATE_PLATFORM_UNSUPPORTED", "desktop update platform is not supported")
	ErrDesktopUpdateMetadata       = infraerrors.New(http.StatusInternalServerError, "DESKTOP_UPDATE_METADATA_INVALID", "published desktop update metadata is invalid")
	ErrDesktopUpdateReasonRequired = infraerrors.BadRequest("DESKTOP_UPDATE_WITHDRAWAL_REASON_REQUIRED", "withdrawal reason is required")
	ErrDesktopUpdateStorageMissing = infraerrors.New(http.StatusServiceUnavailable, "DESKTOP_UPDATE_STORAGE_NOT_CONFIGURED", "desktop update object storage is not configured")
	ErrDesktopUpdateUploadFailed   = infraerrors.New(http.StatusBadGateway, "DESKTOP_UPDATE_UPLOAD_FAILED", "desktop update artifact upload failed")
	ErrDesktopUpdateUploadTooLarge = infraerrors.New(http.StatusRequestEntityTooLarge, "DESKTOP_UPDATE_UPLOAD_TOO_LARGE", "desktop update artifact exceeds the upload limit")

	desktopUpdateVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	desktopUpdatePlatforms      = []string{DesktopUpdateDarwinARM64, DesktopUpdateDarwinX64, DesktopUpdateWindowsX64}
)

type DesktopUpdateArtifact struct {
	URL       string `json:"url"`
	Signature string `json:"signature"`
	ObjectKey string `json:"object_key"`
	FileName  string `json:"file_name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

type DesktopUpdateArtifacts map[string]DesktopUpdateArtifact

type DesktopUpdateRelease struct {
	ID               int64
	PublicID         string
	Version          string
	Notes            string
	Artifacts        DesktopUpdateArtifacts
	Status           string
	CreatedBy        *int64
	UpdatedBy        *int64
	PublishedBy      *int64
	WithdrawnBy      *int64
	PublishedAt      *time.Time
	WithdrawnAt      *time.Time
	WithdrawalReason *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DesktopUpdateDraftInput struct {
	Version   string
	Notes     string
	Artifacts DesktopUpdateArtifacts
	ActorID   int64
}

type DesktopUpdateListFilters struct {
	Status string
}

type DesktopUpdateCheckResult struct {
	Version   string    `json:"version"`
	Notes     string    `json:"notes"`
	PubDate   time.Time `json:"pub_date"`
	URL       string    `json:"url"`
	Signature string    `json:"signature"`
}

type DesktopUpdateRepository interface {
	Create(ctx context.Context, release *DesktopUpdateRelease) error
	List(ctx context.Context, params pagination.PaginationParams, filters DesktopUpdateListFilters) ([]DesktopUpdateRelease, *pagination.PaginationResult, error)
	Get(ctx context.Context, publicID string) (*DesktopUpdateRelease, error)
	UpdateDraft(ctx context.Context, publicID string, input DesktopUpdateDraftInput) (*DesktopUpdateRelease, error)
	Publish(ctx context.Context, publicID string, actorID int64, publishedAt time.Time) (*DesktopUpdateRelease, error)
	Withdraw(ctx context.Context, publicID string, actorID int64, reason string, withdrawnAt time.Time) (*DesktopUpdateRelease, error)
	ListPublished(ctx context.Context) ([]DesktopUpdateRelease, error)
}

type DesktopUpdateArtifactStorage interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) (string, error)
}

type DesktopUpdateArtifactStorageFactory func(context.Context, *config.DesktopUpdateStorageConfig) (DesktopUpdateArtifactStorage, error)

type DesktopUpdateService struct {
	repo           DesktopUpdateRepository
	now            func() time.Time
	storageFactory DesktopUpdateArtifactStorageFactory
	storageConfig  config.DesktopUpdateStorageConfig
}

func NewDesktopUpdateService(repo DesktopUpdateRepository) *DesktopUpdateService {
	return &DesktopUpdateService{repo: repo, now: time.Now}
}

func ProvideDesktopUpdateService(repo DesktopUpdateRepository, factory DesktopUpdateArtifactStorageFactory, cfg *config.Config) *DesktopUpdateService {
	service := NewDesktopUpdateService(repo)
	if cfg != nil {
		service.storageConfig = cfg.DesktopUpdateStorage
	}
	service.storageFactory = factory
	return service
}

func (s *DesktopUpdateService) UploadArtifact(
	ctx context.Context,
	publicID, platform, fileName, contentType string,
	size int64,
	body io.Reader,
) (*DesktopUpdateArtifact, error) {
	release, err := s.Get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if release.Status != DesktopUpdateStatusDraft {
		return nil, ErrDesktopUpdateInvalidState
	}
	platform, ok := desktopUpdatePlatformFromKey(platform)
	if !ok {
		return nil, ErrDesktopUpdateUnsupported.WithMetadata(map[string]string{"platform": strings.TrimSpace(platform)})
	}
	fileName, err = normalizeDesktopUpdateArtifactFileName(platform, fileName)
	if err != nil {
		return nil, err
	}
	if body == nil || size <= 0 {
		return nil, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "file"})
	}
	maxUploadBytes := s.storageConfig.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = 200 << 20
	}
	if size > maxUploadBytes {
		return nil, ErrDesktopUpdateUploadTooLarge.WithMetadata(map[string]string{"max_bytes": fmt.Sprintf("%d", maxUploadBytes)})
	}
	if !s.storageConfig.IsConfigured() || s.storageFactory == nil {
		return nil, ErrDesktopUpdateStorageMissing
	}
	publicBase, err := url.Parse(strings.TrimSpace(s.storageConfig.PublicBaseURL))
	if err != nil || publicBase.Scheme != "https" || publicBase.Host == "" || publicBase.User != nil || publicBase.Fragment != "" {
		return nil, ErrDesktopUpdateStorageMissing.WithMetadata(map[string]string{"field": "desktop_update_storage.public_base_url"})
	}

	storage, err := s.storageFactory(ctx, &s.storageConfig)
	if err != nil {
		return nil, ErrDesktopUpdateUploadFailed.WithCause(err)
	}
	uploadID, err := GenerateDesktopPublicID("artifact")
	if err != nil {
		return nil, ErrDesktopUpdateUploadFailed.WithCause(err)
	}
	key := buildDesktopUpdateObjectKey(s.storageConfig.Prefix, release.Version, platform, uploadID, fileName)
	hasher := sha256.New()
	artifactURL, err := storage.Upload(ctx, key, normalizeDesktopUpdateContentType(contentType), io.TeeReader(body, hasher), size)
	if err != nil {
		return nil, ErrDesktopUpdateUploadFailed.WithCause(err)
	}
	artifact := &DesktopUpdateArtifact{
		URL: artifactURL, ObjectKey: key, FileName: fileName, Size: size,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}
	normalized, err := normalizeDesktopUpdateArtifact(platform, *artifact, false)
	if err != nil {
		return nil, ErrDesktopUpdateUploadFailed.WithCause(err)
	}
	return &normalized, nil
}

func (s *DesktopUpdateService) CreateDraft(ctx context.Context, input DesktopUpdateDraftInput) (*DesktopUpdateRelease, error) {
	normalized, err := normalizeDesktopUpdateDraft(input)
	if err != nil {
		return nil, err
	}
	if err := s.validateStoredArtifacts(normalized.Version, normalized.Artifacts); err != nil {
		return nil, err
	}
	publicID, err := GenerateDesktopPublicID("upd")
	if err != nil {
		return nil, err
	}
	release := &DesktopUpdateRelease{
		PublicID: publicID, Version: normalized.Version, Notes: normalized.Notes,
		Artifacts: normalized.Artifacts, Status: DesktopUpdateStatusDraft,
	}
	if normalized.ActorID > 0 {
		release.CreatedBy = &normalized.ActorID
		release.UpdatedBy = &normalized.ActorID
	}
	if err := s.repo.Create(ctx, release); err != nil {
		return nil, err
	}
	return release, nil
}

func (s *DesktopUpdateService) UpdateDraft(ctx context.Context, publicID string, input DesktopUpdateDraftInput) (*DesktopUpdateRelease, error) {
	normalized, err := normalizeDesktopUpdateDraft(input)
	if err != nil {
		return nil, err
	}
	if err := s.validateStoredArtifacts(normalized.Version, normalized.Artifacts); err != nil {
		return nil, err
	}
	return s.repo.UpdateDraft(ctx, strings.TrimSpace(publicID), normalized)
}

func (s *DesktopUpdateService) List(ctx context.Context, params pagination.PaginationParams, filters DesktopUpdateListFilters) ([]DesktopUpdateRelease, *pagination.PaginationResult, error) {
	filters.Status = strings.TrimSpace(filters.Status)
	if filters.Status != "" && filters.Status != DesktopUpdateStatusDraft && filters.Status != DesktopUpdateStatusPublished && filters.Status != DesktopUpdateStatusWithdrawn {
		return nil, nil, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "status"})
	}
	return s.repo.List(ctx, params, filters)
}

func (s *DesktopUpdateService) Get(ctx context.Context, publicID string) (*DesktopUpdateRelease, error) {
	return s.repo.Get(ctx, strings.TrimSpace(publicID))
}

func (s *DesktopUpdateService) Publish(ctx context.Context, publicID string, actorID int64) (*DesktopUpdateRelease, error) {
	return s.repo.Publish(ctx, strings.TrimSpace(publicID), actorID, s.now().UTC())
}

func (s *DesktopUpdateService) Withdraw(ctx context.Context, publicID string, actorID int64, reason string) (*DesktopUpdateRelease, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return nil, ErrDesktopUpdateReasonRequired
	}
	return s.repo.Withdraw(ctx, strings.TrimSpace(publicID), actorID, reason, s.now().UTC())
}

func (s *DesktopUpdateService) Check(ctx context.Context, target, arch, currentVersion string) (*DesktopUpdateCheckResult, bool, error) {
	version, err := NormalizeDesktopUpdateVersion(currentVersion)
	if err != nil {
		return nil, false, err
	}
	platform, ok := desktopUpdatePlatform(target, arch)
	if !ok {
		return nil, false, ErrDesktopUpdateUnsupported.WithMetadata(map[string]string{"target": strings.TrimSpace(target), "arch": strings.TrimSpace(arch)})
	}
	releases, err := s.repo.ListPublished(ctx)
	if err != nil {
		return nil, false, err
	}
	if len(releases) == 0 {
		return nil, false, nil
	}
	var latest *DesktopUpdateRelease
	for i := range releases {
		candidate := &releases[i]
		if _, err := NormalizeDesktopUpdateVersion(candidate.Version); err != nil {
			return nil, false, ErrDesktopUpdateMetadata.WithCause(err)
		}
		if candidate.PublishedAt == nil {
			return nil, false, ErrDesktopUpdateMetadata
		}
		if _, err := NormalizeDesktopUpdateArtifacts(candidate.Artifacts); err != nil {
			return nil, false, ErrDesktopUpdateMetadata.WithCause(err)
		}
		if latest == nil || semver.Compare("v"+candidate.Version, "v"+latest.Version) > 0 {
			latest = candidate
		}
	}
	if latest == nil || semver.Compare("v"+latest.Version, "v"+version) <= 0 {
		return nil, false, nil
	}
	artifact := latest.Artifacts[platform]
	return &DesktopUpdateCheckResult{
		Version: latest.Version, Notes: latest.Notes, PubDate: latest.PublishedAt.UTC(),
		URL: artifact.URL, Signature: artifact.Signature,
	}, true, nil
}

func NormalizeDesktopUpdateVersion(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > 64 || !desktopUpdateVersionPattern.MatchString(value) || !semver.IsValid("v"+value) {
		return "", ErrDesktopUpdateInvalidVersion.WithMetadata(map[string]string{"field": "version"})
	}
	return value, nil
}

func NormalizeDesktopUpdateArtifacts(value DesktopUpdateArtifacts) (DesktopUpdateArtifacts, error) {
	return normalizeDesktopUpdateArtifacts(value, true)
}

func normalizeDesktopUpdateDraftArtifacts(value DesktopUpdateArtifacts) (DesktopUpdateArtifacts, error) {
	return normalizeDesktopUpdateArtifacts(value, false)
}

func normalizeDesktopUpdateArtifacts(value DesktopUpdateArtifacts, requireComplete bool) (DesktopUpdateArtifacts, error) {
	if len(value) != len(desktopUpdatePlatforms) {
		return nil, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "artifacts"})
	}
	normalized := make(DesktopUpdateArtifacts, len(desktopUpdatePlatforms))
	for _, platform := range desktopUpdatePlatforms {
		artifact, ok := value[platform]
		if !ok {
			return nil, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "artifacts." + platform})
		}
		artifact, err := normalizeDesktopUpdateArtifact(platform, artifact, requireComplete)
		if err != nil {
			return nil, err
		}
		normalized[platform] = artifact
	}
	return normalized, nil
}

func normalizeDesktopUpdateArtifact(platform string, artifact DesktopUpdateArtifact, requireComplete bool) (DesktopUpdateArtifact, error) {
	field := "artifacts." + platform
	artifact.URL = strings.TrimSpace(artifact.URL)
	artifact.Signature = strings.TrimSpace(artifact.Signature)
	artifact.ObjectKey = strings.TrimSpace(artifact.ObjectKey)
	artifact.FileName = strings.TrimSpace(artifact.FileName)
	artifact.SHA256 = strings.ToLower(strings.TrimSpace(artifact.SHA256))

	metadataEmpty := artifact.URL == "" && artifact.ObjectKey == "" && artifact.FileName == "" && artifact.Size == 0 && artifact.SHA256 == ""
	if !metadataEmpty {
		parsed, err := url.Parse(artifact.URL)
		if err != nil || len(artifact.URL) > desktopUpdateMaxURL || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".url"})
		}
		if len(artifact.ObjectKey) > desktopUpdateMaxObjectKey || strings.HasPrefix(artifact.ObjectKey, "/") || strings.Contains(artifact.ObjectKey, "\\") || path.Clean(artifact.ObjectKey) != artifact.ObjectKey || strings.Contains(artifact.ObjectKey, "../") {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".object_key"})
		}
		if artifact.FileName == "" || len(artifact.FileName) > desktopUpdateMaxFileName || strings.ContainsAny(artifact.FileName, "/\\") || path.Base(artifact.FileName) != artifact.FileName {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".file_name"})
		}
		if _, err := normalizeDesktopUpdateArtifactFileName(platform, artifact.FileName); err != nil {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".file_name"})
		}
		if artifact.Size <= 0 {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".size"})
		}
		if len(artifact.SHA256) != desktopUpdateSHA256Length {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".sha256"})
		}
		if _, err := hex.DecodeString(artifact.SHA256); err != nil {
			return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".sha256"})
		}
	} else if requireComplete {
		return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".url"})
	}
	if len(artifact.Signature) > desktopUpdateMaxSignature || (requireComplete && artifact.Signature == "") {
		return DesktopUpdateArtifact{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": field + ".signature"})
	}
	return artifact, nil
}

func normalizeDesktopUpdateArtifactFileName(platform, fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" || len(fileName) > desktopUpdateMaxFileName || strings.ContainsAny(fileName, "/\\") || path.Base(fileName) != fileName {
		return "", ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "file_name"})
	}
	lowerName := strings.ToLower(fileName)
	valid := (platform == DesktopUpdateDarwinARM64 || platform == DesktopUpdateDarwinX64) && strings.HasSuffix(lowerName, ".app.tar.gz")
	valid = valid || platform == DesktopUpdateWindowsX64 && strings.HasSuffix(lowerName, ".nsis.zip")
	if !valid {
		return "", ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "file_name"})
	}
	return fileName, nil
}

func buildDesktopUpdateObjectKey(prefix, version, platform, uploadID, fileName string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "desktop-updates"
	}
	return path.Join(prefix, version, platform, uploadID+"-"+sanitizeDesktopUpdateObjectFileName(fileName))
}

func sanitizeDesktopUpdateObjectFileName(fileName string) string {
	var builder strings.Builder
	for _, char := range fileName {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '.', char == '-', char == '_':
			_, _ = builder.WriteRune(char)
		default:
			_ = builder.WriteByte('_')
		}
	}
	return builder.String()
}

func normalizeDesktopUpdateContentType(contentType string) string {
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	if contentType == "" || len(contentType) > 128 {
		return "application/octet-stream"
	}
	return contentType
}

func desktopUpdatePlatformFromKey(platform string) (string, bool) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	for _, candidate := range desktopUpdatePlatforms {
		if platform == candidate {
			return candidate, true
		}
	}
	return "", false
}

func (s *DesktopUpdateService) validateStoredArtifacts(version string, artifacts DesktopUpdateArtifacts) error {
	for platform, artifact := range artifacts {
		if artifact.URL == "" {
			continue
		}
		if !s.storageConfig.IsConfigured() {
			return ErrDesktopUpdateStorageMissing
		}
		maxUploadBytes := s.storageConfig.MaxUploadBytes
		if maxUploadBytes <= 0 {
			maxUploadBytes = 200 << 20
		}
		if artifact.Size > maxUploadBytes {
			return ErrDesktopUpdateUploadTooLarge.WithMetadata(map[string]string{"max_bytes": fmt.Sprintf("%d", maxUploadBytes)})
		}
		prefix := strings.Trim(strings.TrimSpace(s.storageConfig.Prefix), "/")
		if prefix == "" {
			prefix = "desktop-updates"
		}
		expectedKeyPrefix := path.Join(prefix, version, platform) + "/artifact_"
		if !strings.HasPrefix(artifact.ObjectKey, expectedKeyPrefix) {
			return ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "artifacts." + platform + ".object_key"})
		}
		expectedURL := strings.TrimRight(strings.TrimSpace(s.storageConfig.PublicBaseURL), "/") + "/" + artifact.ObjectKey
		if artifact.URL != expectedURL {
			return ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "artifacts." + platform + ".url"})
		}
	}
	return nil
}

func CompareDesktopUpdateVersions(left, right string) (int, error) {
	left, err := NormalizeDesktopUpdateVersion(left)
	if err != nil {
		return 0, err
	}
	right, err = NormalizeDesktopUpdateVersion(right)
	if err != nil {
		return 0, err
	}
	return semver.Compare("v"+left, "v"+right), nil
}

func normalizeDesktopUpdateDraft(input DesktopUpdateDraftInput) (DesktopUpdateDraftInput, error) {
	version, err := NormalizeDesktopUpdateVersion(input.Version)
	if err != nil {
		return DesktopUpdateDraftInput{}, err
	}
	notes := strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(notes) > desktopUpdateMaxNotes {
		return DesktopUpdateDraftInput{}, ErrDesktopUpdateInvalidFields.WithMetadata(map[string]string{"field": "notes"})
	}
	artifacts, err := normalizeDesktopUpdateDraftArtifacts(input.Artifacts)
	if err != nil {
		return DesktopUpdateDraftInput{}, err
	}
	input.Version, input.Notes, input.Artifacts = version, notes, artifacts
	return input, nil
}

func desktopUpdatePlatform(target, arch string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(target)) + "-" + strings.ToLower(strings.TrimSpace(arch))
	switch key {
	case DesktopUpdateDarwinARM64, DesktopUpdateDarwinX64, DesktopUpdateWindowsX64:
		return key, true
	default:
		return "", false
	}
}
