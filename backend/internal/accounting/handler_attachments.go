package accounting

import (
	"database/sql"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) intent(c *gin.Context) {
	var a Attachment
	if c.ShouldBindJSON(&a) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	org, err := h.resolveOrg(c, a.OrganizationID)
	if response.ErrorFrom(c, err) {
		return
	}
	a.Org = org
	h.run(c, Operation{Org: org, Kind: "attachments", Action: "intent", Payload: a}, func(tx *sql.Tx) (any, error) { return h.engine.Intent(c.Request.Context(), tx, a, actor(c)) })
}
func (h *Handler) attachment(c *gin.Context) *Attachment {
	if !idValid(c, c.Param("id")) {
		return nil
	}
	a, err := lookupAttachment(c.Request.Context(), h.engine.db, c.Param("id"), orgScope(c), c.Request.Method == "DELETE")
	if response.ErrorFrom(c, err) {
		return nil
	}
	return a
}
func (h *Handler) upload(c *gin.Context) {
	a := h.attachment(c)
	if a == nil {
		return
	}
	body, err := readUpload(c.Request.Body)
	if response.ErrorFrom(c, err) {
		return
	}
	h.run(c, Operation{Org: a.Org, Kind: "attachments", Object: a.ID, Action: "upload", Payload: map[string]string{"sha256": hash(body)}}, func(tx *sql.Tx) (any, error) {
		current, err := attachment(c.Request.Context(), tx, a.ID, a.Org)
		if err != nil {
			return nil, err
		}
		err = h.engine.Upload(c.Request.Context(), tx, current, body)
		return current, err
	})
}
func (h *Handler) complete(c *gin.Context) {
	a := h.attachment(c)
	if a == nil {
		return
	}
	h.run(c, Operation{Org: a.Org, Kind: "attachments", Object: a.ID, Action: "complete", Payload: map[string]string{"id": a.ID}}, func(tx *sql.Tx) (any, error) {
		current, err := attachment(c.Request.Context(), tx, a.ID, a.Org)
		if err != nil {
			return nil, err
		}
		err = completeAttachment(c.Request.Context(), tx, current)
		return current, err
	})
}
func (h *Handler) download(c *gin.Context) {
	a := h.attachment(c)
	if a == nil {
		return
	}
	value, err := h.engine.Download(c.Request.Context(), a, actor(c), requestID(c), orgScope(c) > 0)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, value)
}
func (h *Handler) deleteAttachment(c *gin.Context) {
	a := h.attachment(c)
	if a == nil {
		return
	}
	h.run(c, Operation{Org: a.Org, Kind: "attachments", Object: a.ID, Action: "delete", Payload: map[string]string{"id": a.ID}}, func(tx *sql.Tx) (any, error) {
		current, err := lookupAttachment(c.Request.Context(), tx, a.ID, a.Org, true)
		if err != nil {
			return nil, err
		}
		if current.DeletedAt != nil {
			return nil, ErrState
		}
		_, err = tx.ExecContext(c.Request.Context(), "UPDATE accounting_attachments SET deleted_at=NOW(),customer_visible=false WHERE id=$1", a.ID)
		return map[string]any{"id": a.ID, "deleted": true}, err
	})
}
func (h *Handler) listAttachments(c *gin.Context) {
	kind, id := c.Query("object_kind"), c.Query("object_id")
	if !idValid(c, id) {
		return
	}
	org := orgScope(c)
	if org == 0 {
		r, err := get(c.Request.Context(), h.engine.db, kind, id, 0, false)
		if response.ErrorFrom(c, err) {
			return
		}
		org = r.Org
	}
	if orgScope(c) > 0 && kind != "transfers" && kind != "vcc-transactions" && kind != "contracts" && kind != "invoices" {
		response.ErrorFrom(c, ErrNotFound)
		return
	}
	value, err := h.engine.Attachments(c.Request.Context(), kind, id, org, orgScope(c) > 0)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, value)
}
func shareable(kind string, r *Record) bool {
	return kind == "contracts" || (kind == "invoices" && detail(r, "invoice_type") == "SALES")
}
func (h *Handler) shareRecord(c *gin.Context, kind string) {
	r := h.record(c, kind)
	if r == nil {
		return
	}
	if !shareable(kind, r) {
		response.ErrorFrom(c, ErrState)
		return
	}
	var in struct {
		Visible bool `json:"customer_visible"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Org: r.Org, Kind: kind, Object: r.ID, Action: "sharing", Payload: in}, func(tx *sql.Tx) (any, error) {
		current, err := get(c.Request.Context(), tx, kind, r.ID, r.Org, true)
		if err != nil {
			return nil, err
		}
		if !shareable(kind, current) {
			return nil, ErrState
		}
		_, err = tx.ExecContext(c.Request.Context(), "UPDATE "+specs[kind].table+" SET customer_visible=$2,shared_at=CASE WHEN $2 THEN NOW() ELSE NULL END,shared_by=CASE WHEN $2 THEN $3::bigint ELSE NULL END,version=version+1 WHERE id=$1", r.ID, in.Visible, actor(c))
		if err != nil {
			return nil, err
		}
		return get(c.Request.Context(), tx, kind, r.ID, r.Org, false)
	})
}
func (h *Handler) shareAttachment(c *gin.Context) {
	a := h.attachment(c)
	if a == nil {
		return
	}
	var in struct {
		Visible bool `json:"customer_visible"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	h.run(c, Operation{Org: a.Org, Kind: "attachments", Object: a.ID, Action: "sharing", Payload: in}, func(tx *sql.Tx) (any, error) {
		current, err := attachment(c.Request.Context(), tx, a.ID, a.Org)
		if err != nil {
			return nil, err
		}
		a = current
		r, err := get(c.Request.Context(), tx, a.ObjectKind, a.ObjectID, a.Org, false)
		if err != nil {
			return nil, err
		}
		allowed := shareable(a.ObjectKind, r) || a.ObjectKind == "transfers" || (a.ObjectKind == "vcc-transactions" && (detail(r, "transaction_type") == "CONSUME" || detail(r, "transaction_type") == "REFUND"))
		if !allowed || a.Status != "COMPLETE" {
			return nil, ErrState
		}
		_, err = tx.ExecContext(c.Request.Context(), "UPDATE accounting_attachments SET customer_visible=$2,shared_at=CASE WHEN $2 THEN NOW() ELSE NULL END,shared_by=CASE WHEN $2 THEN $3::bigint ELSE NULL END WHERE id=$1 AND deleted_at IS NULL", a.ID, in.Visible, actor(c))
		a.CustomerVisible = in.Visible
		return a, err
	})
}
func safeText(raw string) string { return strings.TrimSpace(raw) }
