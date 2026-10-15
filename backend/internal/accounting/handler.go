package accounting

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ engine *Engine }

func NewHandler(engine *Engine) *Handler { return &Handler{engine: engine} }
func (h *Handler) RegisterAdmin(parent *gin.RouterGroup) {
	g := parent.Group("/accounting")
	g.Use(requireAccountingPermission)
	g.Use(middleware.StrictBodyLimit(1 << 20))
	g.GET("/summary", h.summary)
	g.GET("/trend", h.trend)
	g.GET("/organizations", h.organizations)
	g.GET("/organizations/:organization_id/summary", h.summary)
	g.GET("/audit-logs", h.auditLogs)
	g.GET("/export", h.export)
	for kind := range specs {
		entity := kind
		if entity == "contracts" || entity == "invoices" {
			g.PATCH("/"+entity+"/:id/sharing", func(c *gin.Context) { h.shareRecord(c, entity) })
		}
		g.GET("/"+entity, func(c *gin.Context) { h.list(c, entity) })
		g.GET("/"+entity+"/:id", func(c *gin.Context) { h.get(c, entity) })
		if entity != "expense-payments" {
			g.POST("/"+entity, func(c *gin.Context) { h.create(c, entity) })
			g.PATCH("/"+entity+"/:id", func(c *gin.Context) { h.update(c, entity) })
		}
		for _, action := range []string{"confirm", "void"} {
			act := action
			g.POST("/"+entity+"/:id/"+act, func(c *gin.Context) { h.lifecycle(c, entity, act) })
		}
	}
	g.POST("/contracts/:id/expire", func(c *gin.Context) { h.lifecycle(c, "contracts", "expire") })
	g.POST("/invoices/:id/verify", func(c *gin.Context) { h.lifecycle(c, "invoices", "confirm") })
	g.POST("/vcc-cards/:id/close", func(c *gin.Context) { h.lifecycle(c, "vcc-cards", "close") })
	g.GET("/vcc-cards/:id/transactions", func(c *gin.Context) { h.list(c, "vcc-transactions") })
	g.POST("/vcc-cards/:id/transactions", func(c *gin.Context) { h.create(c, "vcc-transactions") })
	g.POST("/transfers/:id/allocations", h.allocate)
	g.GET("/transfers/:id/allocations", h.allocationHistory)
	g.GET("/vcc-cards/:id/allocations", h.allocationHistory)
	g.POST("/expenses/:id/payments", h.pay)
	g.GET("/reconciliation", h.reconciliation)
	g.POST("/reconciliation/match", func(c *gin.Context) { h.reconcile(c, "match") })
	g.POST("/reconciliation/exclude", func(c *gin.Context) { h.reconcile(c, "exclude") })
	g.POST("/reconciliation/adjustments", h.adjust)
	g.GET("/attachments", h.listAttachments)
	g.POST("/attachments/upload-intents", h.intent)
	// Binary uploads have a separate limit instead of the JSON group's limit.
	upload := parent.Group("/accounting")
	upload.Use(requireAccountingPermission)
	upload.PUT("/attachments/:id/content", middleware.StrictBodyLimit(MaxAttachmentBytes), h.upload)
	g.POST("/attachments/:id/complete", h.complete)
	g.GET("/attachments/:id/download-url", h.download)
	g.DELETE("/attachments/:id", h.deleteAttachment)
	g.PATCH("/attachments/:id/sharing", h.shareAttachment)
}
func (h *Handler) RegisterOrganization(parent *gin.RouterGroup) {
	if !h.engine.desktopEnabled {
		return
	}
	g := parent.Group("/organization/accounting")
	g.Use(func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "Web login required")
			c.Abort()
			return
		}
		org, err := h.engine.desktop.GetManagedOrganization(c.Request.Context(), subject.UserID)
		if response.ErrorFrom(c, err) {
			c.Abort()
			return
		}
		c.Set("accounting_org", org.ID)
		c.Set("accounting_permission", PermissionOrganizationRead)
		c.Header("Cache-Control", "no-store")
		c.Next()
	})
	g.GET("/summary", h.summary)
	g.GET("/trend", h.trend)
	for _, kind := range []string{"transfers", "contracts", "invoices"} {
		entity := kind
		g.GET("/"+entity, func(c *gin.Context) { h.list(c, entity) })
	}
	g.GET("/vcc-card", func(c *gin.Context) { h.list(c, "vcc-cards") })
	g.GET("/vcc-card/transactions", func(c *gin.Context) { h.list(c, "vcc-transactions") })
	g.GET("/attachments", h.listAttachments)
	g.GET("/attachments/:id/download-url", h.download)
}
func actor(c *gin.Context) int64 {
	s, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return s.UserID
}
func orgScope(c *gin.Context) int64 { v, _ := c.Get("accounting_org"); org, _ := v.(int64); return org }
func requestID(c *gin.Context) string {
	v, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	return v
}
func idValid(c *gin.Context, id string) bool {
	if _, err := uuid.Parse(id); err != nil {
		response.ErrorFrom(c, ErrNotFound)
		return false
	}
	return true
}
func filter(c *gin.Context) Filter {
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
	f := Filter{Org: orgScope(c), OrganizationID: c.Query("organization_id"), Currency: c.Query("currency"), Status: c.Query("status"), Search: c.Query("search"), From: c.Query("from"), To: c.Query("to"), Page: page, Size: size, Customer: orgScope(c) > 0}
	if f.Customer {
		f.OrganizationID = ""
		f.Search = ""
	}
	if v := c.Param("organization_id"); v != "" {
		f.OrganizationID = v
	}
	return f
}
func (h *Handler) resolveOrg(c *gin.Context, public string) (int64, error) {
	var id int64
	err := h.engine.db.QueryRowContext(c.Request.Context(), "SELECT id FROM desktop_organizations WHERE public_id=$1 AND deleted_at IS NULL", public).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return id, err
}
func (h *Handler) run(c *gin.Context, op Operation, fn func(*sql.Tx) (any, error)) {
	op.Actor = actor(c)
	op.Key = c.GetHeader("Idempotency-Key")
	op.RequestID = requestID(c)
	body, replayed, err := h.engine.write(c.Request.Context(), op, fn)
	if response.ErrorFrom(c, err) {
		return
	}
	if replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, json.RawMessage(body))
}
func (h *Handler) list(c *gin.Context, kind string) {
	f := filter(c)
	if c.Param("id") != "" {
		if !idValid(c, c.Param("id")) {
			return
		}
		f.ParentID = c.Param("id")
	}
	if c.Query("card_id") != "" {
		if !idValid(c, c.Query("card_id")) {
			return
		}
		f.ParentID = c.Query("card_id")
	}
	items, total, err := h.engine.List(c.Request.Context(), kind, f)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, total, f.Page, f.Size)
}
func (h *Handler) get(c *gin.Context, kind string) {
	if !idValid(c, c.Param("id")) {
		return
	}
	r, err := get(c.Request.Context(), h.engine.db, kind, c.Param("id"), 0, false)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, r)
}
func (h *Handler) create(c *gin.Context, kind string) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	if kind == "vcc-transactions" && c.Param("id") != "" {
		in.ParentID = c.Param("id")
		parent, err := get(c.Request.Context(), h.engine.db, "vcc-cards", in.ParentID, 0, false)
		if response.ErrorFrom(c, err) {
			return
		}
		in.OrganizationID = parent.OrganizationID
	}
	org, err := h.resolveOrg(c, in.OrganizationID)
	if response.ErrorFrom(c, err) {
		return
	}
	h.run(c, Operation{Org: org, Kind: kind, Action: "create", Payload: in}, func(tx *sql.Tx) (any, error) {
		return h.engine.Create(c.Request.Context(), tx, kind, in, org, actor(c))
	})
}
func (h *Handler) record(c *gin.Context, kind string) *Record {
	if !idValid(c, c.Param("id")) {
		return nil
	}
	r, err := get(c.Request.Context(), h.engine.db, kind, c.Param("id"), 0, false)
	if response.ErrorFrom(c, err) {
		return nil
	}
	return r
}
func (h *Handler) update(c *gin.Context, kind string) {
	old := h.record(c, kind)
	if old == nil {
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Org: old.Org, Kind: kind, Object: old.ID, Action: "update", Payload: in}, func(tx *sql.Tx) (any, error) {
		r, err := get(c.Request.Context(), tx, kind, old.ID, old.Org, true)
		if err != nil {
			return nil, err
		}
		return h.engine.Update(c.Request.Context(), tx, kind, r, in)
	})
}
func (h *Handler) lifecycle(c *gin.Context, kind, action string) {
	old := h.record(c, kind)
	if old == nil {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Org: old.Org, Kind: kind, Object: old.ID, Action: action, Payload: in}, func(tx *sql.Tx) (any, error) {
		r, err := get(c.Request.Context(), tx, kind, old.ID, old.Org, true)
		if err != nil {
			return nil, err
		}
		switch action {
		case "confirm":
			err = h.engine.Confirm(c.Request.Context(), tx, kind, r)
		case "void":
			err = h.engine.Void(c.Request.Context(), tx, kind, r, strings.TrimSpace(in.Reason), actor(c))
		case "expire":
			if r.Status != "SIGNED" {
				return nil, ErrState
			}
			r.Status = "EXPIRED"
			err = save(c.Request.Context(), tx, kind, r)
		case "close":
			err = h.engine.CloseCard(c.Request.Context(), tx, r)
		}
		return r, err
	})
}
func (h *Handler) allocate(c *gin.Context) {
	r := h.record(c, "transfers")
	if r == nil {
		return
	}
	var in struct {
		CardID string `json:"card_id"`
		Amount string `json:"amount"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: "transfers", Object: r.ID, Action: "allocate", Payload: in}, func(tx *sql.Tx) (any, error) {
		locked, err := get(c.Request.Context(), tx, "transfers", r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		return h.engine.Allocate(c.Request.Context(), tx, locked, in.CardID, in.Amount, actor(c))
	})
}
func (h *Handler) pay(c *gin.Context) {
	r := h.record(c, "expenses")
	if r == nil {
		return
	}
	var in Input
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: "expenses", Object: r.ID, Action: "pay", Payload: in}, func(tx *sql.Tx) (any, error) {
		locked, err := get(c.Request.Context(), tx, "expenses", r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		return h.engine.Pay(c.Request.Context(), tx, locked, in, actor(c))
	})
}
func (h *Handler) summary(c *gin.Context) {
	value, err := h.engine.Summary(c.Request.Context(), filter(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, value)
}
func (h *Handler) organizations(c *gin.Context) {
	value, err := h.engine.Organizations(c.Request.Context(), filter(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, value)
}
func (h *Handler) export(c *gin.Context) {
	kind := c.Query("kind")
	if _, ok := specs[kind]; !ok {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	f := filter(c)
	f.Size = 100
	f.Page = 1
	out := []*Record{}
	for {
		items, total, err := h.engine.List(c.Request.Context(), kind, f)
		if response.ErrorFrom(c, err) {
			return
		}
		out = append(out, items...)
		if int64(len(out)) >= total {
			break
		}
		f.Page++
		if len(out) > 100000 {
			response.ErrorFrom(c, ErrInvalid)
			return
		}
	}
	if len(out) > 0 {
		if err := audit(c.Request.Context(), h.engine.db, out[0].Org, kind, "", "export", actor(c), requestID(c), map[string]any{"count": len(out)}); response.ErrorFrom(c, err) {
			return
		}
	}
	c.Header("Content-Disposition", `attachment; filename="accounting.csv"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte("\xef\xbb\xbf"))
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"id", "organization_id", "status", "currency", "amount", "occurred_at"})
	for _, r := range out {
		_ = writer.Write([]string{r.ID, r.OrganizationID, r.Status, r.Currency, r.Amount, r.OccurredAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	writer.Flush()
}

func (h *Handler) allocationHistory(c *gin.Context) {
	kind := "transfers"
	if strings.Contains(c.FullPath(), "/vcc-cards/") {
		kind = "vcc-cards"
	}
	record := h.record(c, kind)
	if record == nil {
		return
	}
	column := "transfer_id"
	if kind == "vcc-cards" {
		column = "card_id"
	}
	rows, err := h.engine.db.QueryContext(c.Request.Context(), "SELECT id::text,transfer_id::text,card_id::text,amount::text,used_amount::text,released_amount::text,closed_at IS NOT NULL FROM accounting_allocations WHERE "+column+"=$1 ORDER BY created_at,id", record.ID)
	if response.ErrorFrom(c, err) {
		return
	}
	defer func() { _ = rows.Close() }()
	items := []Allocation{}
	for rows.Next() {
		var a Allocation
		if err = rows.Scan(&a.ID, &a.TransferID, &a.CardID, &a.Amount, &a.Used, &a.Released, &a.Closed); response.ErrorFrom(c, err) {
			return
		}
		items = append(items, a)
	}
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	response.Success(c, items)
}
