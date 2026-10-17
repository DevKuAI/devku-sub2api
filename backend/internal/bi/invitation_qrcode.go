package bi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

const invitationPage = "pages/invitation/index"

var invitationScenePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`)

// InvitationCredential accepts exactly one credential. The column names returned
// by lookup are constants, never client-provided SQL identifiers.
type InvitationCredential struct {
	Token *string `json:"token,omitempty"`
	Scene *string `json:"scene,omitempty"`
}

// Raw fields distinguish an omitted credential from an explicitly supplied null.
type invitationCredentialInput struct {
	Token json.RawMessage `json:"token"`
	Scene json.RawMessage `json:"scene"`
}

func (in invitationCredentialInput) credential() (InvitationCredential, error) {
	result := InvitationCredential{}
	if (len(in.Token) == 0) == (len(in.Scene) == 0) {
		return result, invalid("credential", "Exactly one of token or scene is required")
	}
	field, raw, target := "token", in.Token, &result.Token
	if len(in.Scene) != 0 {
		field, raw, target = "scene", in.Scene, &result.Scene
	}
	if err := json.Unmarshal(raw, target); err != nil || *target == nil {
		return InvitationCredential{}, invalid(field, "Expected a non-null string")
	}
	return result, nil
}

func (c InvitationCredential) lookup() (column, hash string, err error) {
	if (c.Token == nil) == (c.Scene == nil) {
		return "", "", invalid("credential", "Exactly one of token or scene is required")
	}
	if c.Scene != nil {
		if !invitationScenePattern.MatchString(*c.Scene) {
			return "", "", invalid("scene", "Scene must be 32 Base64URL characters")
		}
		return "i.scene_hash", tokenHash(*c.Scene), nil
	}
	if len(*c.Token) < 32 || len(*c.Token) > 128 {
		return "", "", ErrNotFound
	}
	return "i.token_hash", tokenHash(*c.Token), nil
}

func newInvitationScene() (string, error) {
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

type WeChatQRCodeGenerator interface {
	UnlimitedQRCode(context.Context, string, string, string) ([]byte, string, error)
}

type InvitationQRCode struct {
	ImageBase64 string    `json:"image_base64"`
	MIMEType    string    `json:"mime_type"`
	Page        string    `json:"page"`
	EnvVersion  string    `json:"env_version"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// GenerateInvitationQRCode is read-only: retrying a failed upstream request does
// not create an invitation or rotate its credentials.
func (s *Service) GenerateInvitationQRCode(ctx context.Context, org, id, scene, envVersion string, actorID int64) (InvitationQRCode, error) {
	result := InvitationQRCode{}
	if !invitationScenePattern.MatchString(scene) {
		return result, invalid("scene", "Scene must be 32 Base64URL characters")
	}
	if envVersion == "" {
		envVersion = "release"
	}
	if envVersion != "release" && envVersion != "trial" && envVersion != "develop" {
		return result, invalid("env_version", "Expected release, trial or develop")
	}
	check := func() error {
		err := s.db.QueryRowContext(ctx, `SELECT i.expires_at FROM bi_invitations i
 JOIN desktop_organizations o ON o.id=i.organization_id
 JOIN users issuer ON issuer.id=i.created_by JOIN users manager ON manager.id=$4
 WHERE o.public_id=$1 AND i.id=$2 AND i.scene_hash=$3 AND i.status='pending' AND i.expires_at>$5
 AND o.status='active' AND o.deleted_at IS NULL
 AND issuer.status='active' AND issuer.deleted_at IS NULL AND (issuer.role='admin' OR issuer.id=o.gateway_user_id)
 AND manager.status='active' AND manager.deleted_at IS NULL AND (manager.role='admin' OR manager.id=o.gateway_user_id)`, org, id, tokenHash(scene), actorID, s.now()).Scan(&result.ExpiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if err := check(); err != nil {
		return result, err
	}
	generator, ok := s.wechat.(WeChatQRCodeGenerator)
	if !ok {
		return result, apiError(503, "UPSTREAM_UNAVAILABLE", "WeChat QR code generation is unavailable")
	}
	data, mime, err := generator.UnlimitedQRCode(ctx, invitationPage, scene, envVersion)
	if err != nil {
		return result, err
	}
	// Recheck after the network call, without holding database locks across it.
	if err = check(); err != nil {
		return InvitationQRCode{}, err
	}
	result.ImageBase64 = base64.StdEncoding.EncodeToString(data)
	result.MIMEType, result.Page, result.EnvVersion = mime, invitationPage, envVersion
	return result, nil
}

func (h *Handler) InvitationQRCode(c *gin.Context) {
	var in struct {
		Scene      string `json:"scene"`
		EnvVersion string `json:"env_version"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	v, err := h.service.GenerateInvitationQRCode(c.Request.Context(), managedOrganization(c), c.Param("invitation_id"), in.Scene, in.EnvVersion, actor(c))
	adminRespond(c, v, err)
}
