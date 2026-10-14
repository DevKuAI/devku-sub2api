package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Import receives only updater bundles, preserving the original signed bytes.
// Publication is deliberately available only through administrator routes.
func (h *DesktopUpdateHandler) Import(c *gin.Context) {
	version, err := service.NormalizeDesktopUpdateVersion(c.Param("version"))
	if response.ErrorFrom(c, err) {
		return
	}
	err = c.Request.ParseMultipartForm(8 << 20)
	if c.Request.MultipartForm != nil {
		defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
	}
	if err != nil {
		if middleware.IsBodyTooLarge(err) {
			response.ErrorFrom(c, service.ErrDesktopUpdateUploadTooLarge)
		} else {
			response.ErrorFrom(c, service.ErrDesktopUpdateInvalidFields)
		}
		return
	}
	form := c.Request.MultipartForm
	suffixes := map[string]string{service.DesktopUpdateWindowsX64: ".nsis.zip", service.DesktopUpdateDarwinARM64: ".app.tar.gz", service.DesktopUpdateDarwinX64: ".app.tar.gz"}
	if len(form.File) == 0 || len(form.File) > len(suffixes) {
		response.ErrorFrom(c, service.ErrDesktopUpdateInvalidFields)
		return
	}
	for platform, files := range form.File {
		suffix, ok := suffixes[platform]
		if !ok || len(files) != 1 || files[0].Size <= 0 || !strings.HasSuffix(files[0].Filename, suffix) ||
			!strings.Contains(files[0].Filename, "_"+version+"_") || strings.TrimSpace(c.PostForm(platform+".signature")) == "" || len(c.PostForm(platform+".signature")) > 8192 {
			response.ErrorFrom(c, service.ErrDesktopUpdateInvalidFields)
			return
		}
	}
	notes := c.PostForm("notes")
	if len([]rune(notes)) > 20000 {
		response.ErrorFrom(c, service.ErrDesktopUpdateInvalidFields)
		return
	}
	release, err := h.updates.ImportDraft(c.Request.Context(), version)
	if response.ErrorFrom(c, err) {
		return
	}
	artifacts := make(service.DesktopUpdateArtifacts, len(form.File))
	for platform, files := range form.File {
		file, openErr := files[0].Open()
		if openErr != nil {
			response.ErrorFrom(c, service.ErrDesktopUpdateInvalidFields)
			return
		}
		artifact, uploadErr := h.updates.UploadArtifact(c.Request.Context(), release.PublicID, platform, files[0].Filename, "application/octet-stream", files[0].Size, file)
		_ = file.Close()
		if response.ErrorFrom(c, uploadErr) {
			return
		}
		artifact.Signature = strings.TrimSpace(c.PostForm(platform + ".signature"))
		artifacts[platform] = *artifact
	}
	release, err = h.updates.UpdateDraft(c.Request.Context(), release.PublicID, service.DesktopUpdateDraftInput{Version: version, Notes: notes, Artifacts: artifacts})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, desktopUpdateFromService(release))
}
