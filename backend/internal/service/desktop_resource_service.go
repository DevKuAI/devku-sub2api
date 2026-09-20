package service

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresource"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

var (
	ErrResourceInvalid    = infraerrors.BadRequest("RESOURCE_PACKAGE_INVALID", "invalid resource package")
	ErrResourceNotFound   = infraerrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	ErrResourceConflict   = infraerrors.Conflict("RESOURCE_VERSION_CONFLICT", "resource version already exists, is older, or has conflicting metadata")
	ErrResourceValidation = infraerrors.New(422, "VALIDATION_FAILED", "invalid resource request")
	ErrResourceTooLarge   = infraerrors.New(413, "PAYLOAD_TOO_LARGE", "resource ZIP exceeds 64 MiB")
	ErrResourceStorage    = infraerrors.New(503, "RESOURCE_STORAGE_UNAVAILABLE", "resource storage is unavailable")
)

type DesktopResourceArtifact struct {
	Platform  string                   `json:"platform"`
	SHA256    string                   `json:"sha256"`
	SizeBytes int64                    `json:"sizeBytes"`
	Manifest  desktopresource.Manifest `json:"manifest"`
	ObjectKey string                   `json:"-"`
	CreatedBy int64                    `json:"createdBy,omitempty"`
	CreatedAt *time.Time               `json:"createdAt,omitempty"`
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

type DesktopResourceRecord struct {
	DesktopResource
	Status       string    `json:"status"`
	StatusReason string    `json:"statusReason"`
	CreatedBy    int64     `json:"createdBy"`
	UpdatedBy    int64     `json:"updatedBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (r DesktopResourceRecord) Public() DesktopResource {
	result := r.DesktopResource
	result.Artifacts = append([]DesktopResourceArtifact{}, r.Artifacts...)
	for i := range result.Artifacts {
		result.Artifacts[i].ObjectKey = ""
		result.Artifacts[i].CreatedBy = 0
		result.Artifacts[i].CreatedAt = nil
	}
	return result
}

type DesktopResourceVersion struct {
	Version     string                    `json:"version"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	SourceURL   string                    `json:"sourceUrl"`
	CreatedBy   int64                     `json:"createdBy"`
	CreatedAt   time.Time                 `json:"createdAt"`
	Artifacts   []DesktopResourceArtifact `json:"artifacts"`
}

type DesktopResourceFilter struct {
	Kind, Search, Key, Status string
	Limit, Offset             int
}

func (f DesktopResourceFilter) Validate() error {
	if (f.Kind != "" && f.Kind != "mcp" && f.Kind != "skill") || (f.Status != "" && f.Status != "active" && f.Status != "disabled") || len(f.Search) > 200 || (f.Key != "" && !desktopresource.KeyPattern.MatchString(f.Key)) || f.Limit < 1 || f.Limit > 100 || f.Offset < 0 || f.Offset > 10000 {
		return ErrResourceValidation
	}
	return nil
}

type DesktopResourceRepository interface {
	Publish(context.Context, desktopresource.Package, DesktopResourceArtifact, int64) (*DesktopResourceRecord, error)
	List(context.Context, DesktopResourceFilter) ([]DesktopResourceRecord, int64, error)
	Get(context.Context, string, bool) (*DesktopResourceRecord, error)
	Versions(context.Context, string, int, int) ([]DesktopResourceVersion, int64, error)
	Artifact(context.Context, string, string, string) (*DesktopResourceArtifact, error)
	SetStatus(context.Context, string, string, string, int64) (*DesktopResourceRecord, error)
}

type DesktopResourceStorage interface {
	Upload(context.Context, string, io.Reader, int64) error
	Presign(context.Context, string, time.Duration) (string, error)
	Open(context.Context, string) (io.ReadCloser, int64, error)
}

const DesktopResourceDownloadURLTTL = 5 * time.Minute

type DesktopResourceDownloadURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
	ExpiresIn int       `json:"expiresIn"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"sizeBytes"`
}

// DownloadURL is called only after administrator/member authorization. URLs are not persisted.
func (s *DesktopResourceService) DownloadURL(ctx context.Context, id, version, platform string) (*DesktopResourceDownloadURL, error) {
	if !desktopresource.VersionPattern.MatchString(version) || (!desktopresource.ValidPlatform(platform, false) && platform != "any") {
		return nil, ErrResourceNotFound
	}
	artifact, err := s.repo.Artifact(ctx, id, version, platform)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(DesktopResourceDownloadURLTTL).Truncate(time.Second)
	url, err := s.storage.Presign(ctx, artifact.ObjectKey, DesktopResourceDownloadURLTTL)
	if err != nil {
		return nil, ErrResourceStorage.WithCause(err)
	}
	return &DesktopResourceDownloadURL{URL: url, ExpiresAt: expiresAt, ExpiresIn: int(DesktopResourceDownloadURLTTL.Seconds()), SHA256: artifact.SHA256, SizeBytes: artifact.SizeBytes}, nil
}

type DesktopResourceService struct {
	repo    DesktopResourceRepository
	storage DesktopResourceStorage
	prefix  string
}

func NewDesktopResourceService(repo DesktopResourceRepository, storage DesktopResourceStorage, cfg *config.Config) *DesktopResourceService {
	prefix := "desktop-updates"
	if cfg != nil && cfg.DesktopUpdateStorage.Prefix != "" {
		prefix = cfg.DesktopUpdateStorage.Prefix
	}
	return &DesktopResourceService{repo: repo, storage: storage, prefix: path.Join(prefix, "resources")}
}

// spool validates before uploading and bounds memory usage regardless of the archive size.
func spoolDesktopResource(body io.Reader) (*os.File, *desktopresource.Package, error) {
	f, err := os.CreateTemp("", "desktop-resource-*.zip")
	if err != nil {
		return nil, nil, ErrResourceStorage.WithCause(err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	n, err := io.Copy(f, io.LimitReader(body, desktopresource.MaxZIPBytes+1))
	var tooLarge *http.MaxBytesError
	if n > desktopresource.MaxZIPBytes || errors.As(err, &tooLarge) {
		return nil, nil, ErrResourceTooLarge
	}
	if err != nil {
		return nil, nil, ErrResourceValidation.WithCause(err)
	}
	pkg, err := desktopresource.Inspect(f.Name())
	if err != nil {
		return nil, nil, ErrResourceInvalid.WithMetadata(map[string]string{"detail": err.Error()})
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, ErrResourceStorage.WithCause(err)
	}
	keep = true
	return f, pkg, nil
}

func closeDesktopResource(f *os.File) { _ = f.Close(); _ = os.Remove(f.Name()) }

type DesktopResourceValidation struct {
	*desktopresource.Package
	Current *DesktopResourceRecord `json:"current"`
}

func (s *DesktopResourceService) Validate(ctx context.Context, body io.Reader) (*DesktopResourceValidation, error) {
	f, pkg, err := spoolDesktopResource(body)
	if err != nil {
		return nil, err
	}
	defer closeDesktopResource(f)
	items, _, err := s.repo.List(ctx, DesktopResourceFilter{Key: pkg.Manifest.Key, Limit: 1})
	if err != nil {
		return nil, err
	}
	result := &DesktopResourceValidation{Package: pkg}
	if len(items) > 0 {
		result.Current = &items[0]
	}
	return result, nil
}

func (s *DesktopResourceService) Publish(ctx context.Context, body io.Reader, actor int64) (*DesktopResourceRecord, error) {
	f, pkg, err := spoolDesktopResource(body)
	if err != nil {
		return nil, err
	}
	defer closeDesktopResource(f)
	key := path.Join(s.prefix, pkg.Manifest.Key, pkg.Manifest.Version, pkg.Manifest.Platform, uuid.NewString()+".zip")
	err = s.storage.Upload(ctx, key, f, pkg.SizeBytes)
	if err != nil {
		return nil, ErrResourceStorage.WithCause(err)
	}
	artifact := DesktopResourceArtifact{Platform: pkg.Manifest.Platform, SHA256: pkg.SHA256, SizeBytes: pkg.SizeBytes, Manifest: pkg.Manifest, ObjectKey: key}
	// A commit error can be ambiguous. Never delete the uploaded object on database errors.
	result, err := s.repo.Publish(ctx, *pkg, artifact, actor)
	if err != nil {
		log.Printf("[DesktopResource] publication requires verification: key=%s sha256=%s", key, pkg.SHA256)
	}
	return result, err
}

func (s *DesktopResourceService) List(ctx context.Context, filter DesktopResourceFilter) ([]DesktopResourceRecord, int64, error) {
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, filter)
}

func (s *DesktopResourceService) Get(ctx context.Context, id string, public bool) (*DesktopResourceRecord, error) {
	return s.repo.Get(ctx, id, public)
}

func (s *DesktopResourceService) Versions(ctx context.Context, id string, limit, offset int) ([]DesktopResourceVersion, int64, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return nil, 0, ErrResourceValidation
	}
	if _, err := s.repo.Get(ctx, id, false); err != nil {
		return nil, 0, err
	}
	return s.repo.Versions(ctx, id, limit, offset)
}

func (s *DesktopResourceService) SetStatus(ctx context.Context, id, status, reason string, actor int64) (*DesktopResourceRecord, error) {
	reason = strings.TrimSpace(reason)
	if (status != "active" && status != "disabled") || (status == "disabled" && reason == "") || len(reason) > 2000 {
		return nil, ErrResourceValidation
	}
	if status == "active" {
		reason = ""
	}
	return s.repo.SetStatus(ctx, id, status, reason, actor)
}

func (s *DesktopResourceService) Download(ctx context.Context, id, version, platform string) (*DesktopResourceArtifact, io.ReadCloser, error) {
	if !desktopresource.VersionPattern.MatchString(version) || (!desktopresource.ValidPlatform(platform, false) && platform != "any") {
		return nil, nil, ErrResourceNotFound
	}
	artifact, err := s.repo.Artifact(ctx, id, version, platform)
	if err != nil {
		return nil, nil, err
	}
	body, size, err := s.storage.Open(ctx, artifact.ObjectKey)
	if err != nil {
		return nil, nil, ErrResourceStorage.WithCause(err)
	}
	if size != artifact.SizeBytes {
		_ = body.Close()
		return nil, nil, ErrResourceStorage
	}
	return artifact, body, nil
}
