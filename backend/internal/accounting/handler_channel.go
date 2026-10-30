package accounting

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounting/channel"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MaxChannelUploadBytes bounds one multipart upload: the files plus form overhead.
const MaxChannelUploadBytes = 64 << 20

var channelContentTypes = map[string]string{
	".csv":  "text/csv",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pdf":  "application/pdf",
}

func (h *Handler) registerChannels(g, parent *gin.RouterGroup) {
	g.GET("/channel-accounts", h.listChannelAccounts)
	g.GET("/channel-accounts/:id", h.channelAccount)
	g.PATCH("/channel-accounts/:id", h.updateChannelAccount)
	g.GET("/channel-accounts/:id/lines", h.channelLines)
	g.GET("/channel-imports", h.listChannelImports)
	g.GET("/channel-imports/:id", h.channelImport)
	g.POST("/channel-imports/:id/revoke", h.revokeChannelImport)
	g.GET("/channel-imports/:id/files/:index/download-url", h.channelFileDownload)
	g.GET("/channel-template", h.channelTemplate)
	upload := parent.Group("/accounting")
	upload.Use(requireAccountingPermission)
	upload.POST("/channel-imports", middleware.StrictBodyLimit(MaxChannelUploadBytes), h.importChannel)
	upload.POST("/channel-imports/convert", middleware.StrictBodyLimit(MaxChannelUploadBytes), h.convertChannel)
	upload.POST("/channel-lines/:id/proofs", middleware.StrictBodyLimit(MaxAttachmentBytes+1<<20), h.addChannelProof)
	g.GET("/channel-proofs/:id/download-url", h.channelProofDownload)
	g.DELETE("/channel-proofs/:id", h.deleteChannelProof)
}

// addChannelProof stores one proof file; the page uploads several files one by one.
func (h *Handler) addChannelProof(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	header, err := c.FormFile("file")
	if err != nil || header.Size <= 0 || header.Size > MaxAttachmentBytes {
		response.ErrorFrom(c, invalid("file"))
		return
	}
	f, err := header.Open()
	if err != nil {
		response.ErrorFrom(c, invalid("file"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxAttachmentBytes+1))
	_ = f.Close()
	name := channelProofName(header.Filename)
	// The content decides the type, so a renamed file cannot pass as an image.
	contentType := http.DetectContentType(data)
	if err != nil || int64(len(data)) > MaxAttachmentBytes || name == "" || !validAttachment(name, contentType) {
		response.ErrorFrom(c, invalid("file"))
		return
	}
	if h.engine.storage == nil || !h.engine.storage.Ready() {
		response.ErrorFrom(c, ErrStorage)
		return
	}
	sum := sha256.Sum256(data)
	upload := channelProofUpload{ID: uuid.NewString(), Filename: name, ContentType: contentType, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}
	upload.ObjectKey = "accounting/channel/proofs/" + upload.ID + "/" + upload.SHA256 + strings.ToLower(fileExt(name))
	if err = h.engine.storage.Upload(c.Request.Context(), upload.ObjectKey, bytes.NewReader(data), upload.Size, contentType); err != nil {
		response.ErrorFrom(c, ErrStorage)
		return
	}
	line := c.Param("id")
	payload := map[string]string{"name": name, "sha256": upload.SHA256}
	h.run(c, Operation{Global: true, Kind: "channel-lines", Object: line, Action: "add-proof", Payload: payload}, func(tx *sql.Tx) (any, error) {
		return h.engine.AddChannelProof(c.Request.Context(), tx, line, upload, actor(c))
	})
}

func (h *Handler) channelProofDownload(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	filename, contentType, key, err := channelProofObject(c.Request.Context(), h.engine.db, c.Param("id"))
	if response.ErrorFrom(c, err) {
		return
	}
	if h.engine.storage == nil || !h.engine.storage.Ready() {
		response.ErrorFrom(c, ErrStorage)
		return
	}
	url, err := h.engine.storage.Presign(c.Request.Context(), key, filename, contentType, 5*time.Minute)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"url": url, "expires_in": 300})
}

func (h *Handler) deleteChannelProof(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	id := c.Param("id")
	h.run(c, Operation{Global: true, Kind: "channel-proofs", Object: id, Action: "delete", Payload: map[string]string{"id": id}}, func(tx *sql.Tx) (any, error) {
		return h.engine.DeleteChannelProof(c.Request.Context(), tx, id, actor(c))
	})
}

func (h *Handler) listChannelAccounts(c *gin.Context) {
	accounts, err := h.engine.ListChannelAccounts(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, accounts)
}

func (h *Handler) channelAccount(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	detail, err := h.engine.ChannelAccountDetail(c.Request.Context(), c.Param("id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, detail)
}

func (h *Handler) updateChannelAccount(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	var in struct {
		Name   string `json:"name"`
		Remark string `json:"remark"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	id := c.Param("id")
	h.run(c, Operation{Global: true, Kind: "channel-accounts", Object: id, Action: "update", Payload: in}, func(tx *sql.Tx) (any, error) {
		return h.engine.UpdateChannelAccount(c.Request.Context(), tx, id, in.Name, in.Remark)
	})
}

func (h *Handler) channelLines(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	page, size := pageParams(c)
	f := ChannelLineFilter{Wallet: c.Query("wallet"), Currency: c.Query("currency"), Type: c.Query("type"), Card: c.Query("card_id"),
		From: c.Query("from"), To: c.Query("to"), Search: c.Query("search"), Page: page, Size: size}
	items, total, err := h.engine.ChannelLines(c.Request.Context(), c.Param("id"), f)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, int64(total), page, size)
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("page_size"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func (h *Handler) listChannelImports(c *gin.Context) {
	page, size := pageParams(c)
	items, total, err := h.engine.ListChannelImports(c.Request.Context(), c.Query("account_id"), page, size)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, int64(total), page, size)
}

func (h *Handler) channelImport(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	im, err := getChannelImport(c.Request.Context(), h.engine.db, c.Param("id"))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, im)
}

func (h *Handler) revokeChannelImport(c *gin.Context) {
	if !idValid(c, c.Param("id")) || !requireAdmin(c) {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 1000 {
		response.ErrorFrom(c, invalid("reason"))
		return
	}
	id := c.Param("id")
	h.run(c, Operation{Global: true, Kind: "channel-imports", Object: id, Action: "revoke", Payload: in}, func(tx *sql.Tx) (any, error) {
		return h.engine.RevokeChannelImport(c.Request.Context(), tx, id, strings.TrimSpace(in.Reason), actor(c))
	})
}

func (h *Handler) channelFileDownload(c *gin.Context) {
	if !idValid(c, c.Param("id")) {
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		response.ErrorFrom(c, ErrNotFound)
		return
	}
	file, err := channelFileObject(c.Request.Context(), h.engine.db, c.Param("id"), index)
	if response.ErrorFrom(c, err) {
		return
	}
	if h.engine.storage == nil || !h.engine.storage.Ready() {
		response.ErrorFrom(c, ErrStorage)
		return
	}
	url, err := h.engine.storage.Presign(c.Request.Context(), file.ObjectKey, file.Name, channelContentTypes[strings.ToLower(fileExt(file.Name))], 5*time.Minute)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"url": url, "expires_in": 300})
}

func fileExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}

func (h *Handler) channelTemplate(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="channel-statement-template.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", channel.WriteCSV(nil))
}

// readChannelUpload reads the multipart "files" field into memory; each file is small enough to parse at once.
func readChannelUpload(c *gin.Context) ([]channel.SourceFile, []ChannelFile, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, nil, ErrInvalid
	}
	headers := form.File["files"]
	if len(headers) == 0 || len(headers) > channel.MaxFiles {
		return nil, nil, invalid("files")
	}
	sources := make([]channel.SourceFile, 0, len(headers))
	meta := make([]ChannelFile, 0, len(headers))
	for _, header := range headers {
		data, readErr := readPart(header)
		if readErr != nil {
			return nil, nil, readErr
		}
		name := strings.TrimSpace(header.Filename)
		if _, ok := channelContentTypes[strings.ToLower(fileExt(name))]; !ok || name == "" || len(name) > 255 {
			return nil, nil, invalid("files")
		}
		sum := sha256.Sum256(data)
		sources = append(sources, channel.SourceFile{Name: name, Data: data})
		meta = append(meta, ChannelFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Size: len(data)})
	}
	return sources, meta, nil
}

func readPart(header *multipart.FileHeader) ([]byte, error) {
	if header.Size <= 0 || header.Size > channel.MaxFileBytes {
		return nil, invalid("files")
	}
	f, err := header.Open()
	if err != nil {
		return nil, ErrInvalid
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, channel.MaxFileBytes+1))
	if err != nil || len(data) > channel.MaxFileBytes {
		return nil, invalid("files")
	}
	return data, nil
}

func (h *Handler) convertChannel(c *gin.Context) {
	sources, _, err := readChannelUpload(c)
	if response.ErrorFrom(c, err) {
		return
	}
	result, err := channel.Convert(sources, c.PostForm("converter"))
	if response.ErrorFrom(c, channelFilesError(err)) {
		return
	}
	// JSON rather than a file, so problems reach the page the same way as for an import.
	response.Success(c, gin.H{"converter": result.Converter, "lines": len(result.Lines), "balances": result.Balances, "csv": string(channel.WriteCSV(result.Lines))})
}

func (h *Handler) importChannel(c *gin.Context) {
	sources, files, err := readChannelUpload(c)
	if response.ErrorFrom(c, err) {
		return
	}
	converter := c.PostForm("converter")
	result, err := channel.Convert(sources, converter)
	if response.ErrorFrom(c, channelFilesError(err)) {
		return
	}
	importID := uuid.NewString()
	// Originals are archived before booking; a failed booking only leaves an unreferenced private object.
	if h.engine.storage != nil && h.engine.storage.Ready() {
		for i := range files {
			key := "accounting/channel/" + importID + "/" + files[i].SHA256 + strings.ToLower(fileExt(files[i].Name))
			contentType := channelContentTypes[strings.ToLower(fileExt(files[i].Name))]
			if uploadErr := h.engine.storage.Upload(c.Request.Context(), key, bytes.NewReader(sources[i].Data), int64(files[i].Size), contentType); uploadErr != nil {
				response.ErrorFrom(c, ErrStorage)
				return
			}
			files[i].ObjectKey = key
		}
	}
	fingerprint := make([]map[string]string, 0, len(files))
	for _, f := range files {
		fingerprint = append(fingerprint, map[string]string{"name": f.Name, "sha256": f.SHA256})
	}
	payload := map[string]any{"converter": converter, "files": fingerprint}
	h.run(c, Operation{Global: true, Kind: "channel-imports", Action: "import", Payload: payload}, func(tx *sql.Tx) (any, error) {
		out, importErr := h.engine.ImportChannel(c.Request.Context(), tx, importID, result, files, actor(c))
		return out, channelFilesError(importErr)
	})
}
