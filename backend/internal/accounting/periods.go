package accounting

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var accountingZone = time.FixedZone("Asia/Shanghai", 8*3600)

type StatementEntry struct {
	Kind   string  `json:"kind"`
	Record *Record `json:"record"`
}

type Position struct {
	Cash string `json:"cash"`
	Card string `json:"card"`
}

func currentPositions(ctx context.Context, q dbQuery, org int64) (map[string]Position, error) {
	rows, err := q.QueryContext(ctx, `WITH positions AS (
 SELECT r.currency,r.amount+COALESCE((SELECT SUM(x.amount) FROM accounting_adjustments x WHERE x.object_kind='transfers' AND x.object_id=r.id),0)-COALESCE((SELECT SUM(a.amount-a.released_amount) FROM accounting_allocations a WHERE a.transfer_id=r.id),0)-COALESCE((SELECT SUM(x.amount+COALESCE(NULLIF(x.details->>'fee_amount','')::numeric,0)) FROM accounting_fx_exchanges x WHERE x.parent_id=r.id AND x.status='CONFIRMED'),0) AS cash,0::numeric AS card FROM accounting_transfers r WHERE r.organization_id=$1 AND r.status='RECEIVED'
 UNION ALL SELECT r.currency,0::numeric,COALESCE((SELECT SUM(a.amount-a.released_amount) FROM accounting_allocations a WHERE a.card_id=r.id),0)-COALESCE((SELECT SUM(CASE WHEN t.details->>'transaction_type'='REFUND' OR (t.details->>'transaction_type'='ADJUSTMENT' AND t.details->>'direction'='CREDIT') THEN -t.amount ELSE t.amount END) FROM accounting_vcc_transactions t WHERE t.parent_id=r.id AND t.status='CONFIRMED'),0) FROM accounting_vcc_cards r WHERE r.organization_id=$1)
 SELECT currency,SUM(cash)::text,SUM(card)::text FROM positions GROUP BY currency`, org)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]Position{}
	for rows.Next() {
		var currency string
		var p Position
		if err = rows.Scan(&currency, &p.Cash, &p.Card); err != nil {
			return nil, err
		}
		out[currency] = p
	}
	return out, rows.Err()
}
func ensurePositionBaseline(ctx context.Context, tx dbQuery, org int64, before map[string]Position) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_position_baselines WHERE organization_id=$1)", org).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	var legacy bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_transfers WHERE organization_id=$1) OR EXISTS(SELECT 1 FROM accounting_vcc_transactions WHERE organization_id=$1) OR EXISTS(SELECT 1 FROM accounting_allocations WHERE organization_id=$1)`, org).Scan(&legacy); err != nil {
		return err
	}
	covered := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	if legacy {
		covered = time.Now().UTC()
	}
	body, err := json.Marshal(before)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO accounting_position_baselines(organization_id,covered_from,balances) VALUES($1,$2,$3)", org, covered, body)
	return err
}
func operationDate(ctx context.Context, q dbQuery, op Operation) (time.Time, error) {
	var payload struct {
		OccurredAt string `json:"occurred_at"`
	}
	body, err := json.Marshal(op.Payload)
	if err != nil {
		return time.Time{}, err
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return time.Time{}, err
	}
	if payload.OccurredAt != "" {
		return workflowTime(payload.OccurredAt)
	}
	if s, ok := specs[op.Kind]; ok && op.Object != "" && (op.Action == "confirm" || op.Action == "update" || op.Action == "void" || op.Action == "adjust") {
		var occurred time.Time
		err = q.QueryRowContext(ctx, "SELECT occurred_at FROM "+s.table+" WHERE id=$1 AND organization_id=$2", op.Object, op.Org).Scan(&occurred)
		return occurred, err
	}
	return time.Now().UTC(), nil
}
func checkOpenPeriod(ctx context.Context, q dbQuery, org int64, date time.Time) error {
	month := date.In(accountingZone).Format("2006-01") + "-01"
	var closed bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accounting_periods WHERE organization_id=$1 AND month=$2::date AND status='CLOSED')", org, month).Scan(&closed)
	if err != nil {
		return err
	}
	if closed {
		return ErrPeriodClosed.WithMetadata(map[string]string{"month": month[:7]})
	}
	return nil
}
func protectPeriod(ctx context.Context, q dbQuery, op Operation, date time.Time) error {
	if op.Kind == "periods" || op.Action == "sharing" {
		return nil
	}
	if err := checkOpenPeriod(ctx, q, op.Org, date); err != nil {
		return err
	}
	if s, ok := specs[op.Kind]; ok && op.Object != "" && (op.Action == "update" || op.Action == "void" || op.Action == "adjust" || op.Action == "confirm") {
		var original time.Time
		if err := q.QueryRowContext(ctx, "SELECT occurred_at FROM "+s.table+" WHERE id=$1", op.Object).Scan(&original); err != nil {
			return err
		}
		return checkOpenPeriod(ctx, q, op.Org, original)
	}
	if op.Action == "void" && (op.Kind == "settlements" || op.Kind == "cost-allocations") {
		table := "accounting_receipt_settlements"
		if op.Kind == "cost-allocations" {
			table = "accounting_cost_allocations"
		}
		var original time.Time
		if err := q.QueryRowContext(ctx, "SELECT occurred_at FROM "+table+" WHERE id=$1", op.Object).Scan(&original); err != nil {
			return err
		}
		return checkOpenPeriod(ctx, q, op.Org, original)
	}
	if op.Kind == "cost-allocations" && op.Action == "create" {
		var in workflowInput
		body, _ := json.Marshal(op.Payload)
		if err := json.Unmarshal(body, &in); err != nil {
			return err
		}
		for _, item := range []struct{ kind, id string }{{"vendor-invoices", in.InvoiceID}, {in.SourceKind, in.SourceID}} {
			if _, ok := specs[item.kind]; !ok {
				return ErrInvalid
			}
			record, err := get(ctx, q, item.kind, item.id, op.Org, false)
			if err != nil {
				return err
			}
			if err = checkOpenPeriod(ctx, q, op.Org, record.OccurredAt); err != nil {
				return err
			}
		}
	}

	if op.Kind == "attachments" && op.Action == "delete" {
		a, err := attachment(ctx, q, op.Object, op.Org)
		if err != nil {
			return err
		}
		if err = checkOpenPeriod(ctx, q, op.Org, a.CreatedAt); err != nil {
			return err
		}
		r, err := get(ctx, q, a.ObjectKind, a.ObjectID, op.Org, false)
		if err != nil {
			return err
		}
		return checkOpenPeriod(ctx, q, op.Org, r.OccurredAt)
	}

	return nil
}
func recordPositionChanges(ctx context.Context, q dbQuery, op Operation, date time.Time, before, after map[string]Position) error {
	currencies := map[string]bool{}
	for c := range before {
		currencies[c] = true
	}
	for c := range after {
		currencies[c] = true
	}
	for currency := range currencies {
		a, b := before[currency], after[currency]
		cash, card := dec(b.Cash).Sub(dec(a.Cash)), dec(b.Card).Sub(dec(a.Card))
		if cash.IsZero() && card.IsZero() {
			continue
		}
		_, err := q.ExecContext(ctx, `INSERT INTO accounting_position_events(id,organization_id,currency,cash_delta,card_delta,occurred_at,object_kind,object_id,action) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuid.NewString(), op.Org, currency, cash.String(), card.String(), date, op.Kind, op.Object, op.Action)
		if err != nil {
			return err
		}
	}
	return nil
}
func positionsAt(ctx context.Context, q dbQuery, org int64, boundary time.Time) (map[string]Position, time.Time, error) {
	var covered time.Time
	var body []byte
	err := q.QueryRowContext(ctx, "SELECT covered_from,balances FROM accounting_position_baselines WHERE organization_id=$1", org).Scan(&covered, &body)
	if err != nil {
		return nil, covered, err
	}
	if boundary.Before(covered) {
		return nil, covered, nil
	}
	out := map[string]Position{}
	if err = json.Unmarshal(body, &out); err != nil {
		return nil, covered, err
	}
	rows, err := q.QueryContext(ctx, "SELECT currency,SUM(cash_delta)::text,SUM(card_delta)::text FROM accounting_position_events WHERE organization_id=$1 AND occurred_at<$2 GROUP BY currency", org, boundary)
	if err != nil {
		return nil, covered, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var currency, cash, card string
		if err = rows.Scan(&currency, &cash, &card); err != nil {
			return nil, covered, err
		}
		p := out[currency]
		out[currency] = Position{Cash: dec(p.Cash).Add(dec(cash)).String(), Card: dec(p.Card).Add(dec(card)).String()}
	}
	return out, covered, rows.Err()
}
func (h *Handler) periods(c *gin.Context) {
	org, ok := h.workflowOrg(c, c.Query("organization_id"))
	if !ok {
		return
	}
	visibility := ""
	if orgScope(c) > 0 {
		visibility = " AND p.status='CLOSED'"
	}
	h.workflowRows(c, `SELECT jsonb_build_object('id',p.id,'month',to_char(p.month,'YYYY-MM'),'status',p.status,'revision',p.revision,'reason',CASE WHEN $2::boolean THEN '' ELSE p.reason END,'statement_id',s.id,'created_at',s.created_at) FROM accounting_periods p LEFT JOIN accounting_statements s ON s.period_id=p.id AND s.revision=p.revision WHERE p.organization_id=$1`+visibility+` ORDER BY p.month DESC`, org, orgScope(c) > 0)
}
func (h *Handler) closePeriod(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	org, ok := h.workflowOrg(c, in.OrganizationID)
	if !ok {
		return
	}
	start, err := time.ParseInLocation("2006-01", in.Month, accountingZone)
	if response.ErrorFrom(c, err) {
		return
	}
	end := start.AddDate(0, 1, 0)
	if end.After(time.Now()) {
		response.ErrorFrom(c, invalid("month_not_finished"))
		return
	}
	h.run(c, Operation{Org: org, Kind: "periods", Action: "close", Payload: in}, func(tx *sql.Tx) (any, error) {
		var earlierOpen bool
		if err := tx.QueryRowContext(c.Request.Context(), "SELECT EXISTS(SELECT 1 FROM accounting_periods WHERE organization_id=$1 AND month<$2 AND status='OPEN')", org, start).Scan(&earlierOpen); err != nil {
			return nil, err
		}
		if earlierOpen {
			return nil, invalid("earlier_period_open")
		}
		var existingStatus string
		err := tx.QueryRowContext(c.Request.Context(), "SELECT status FROM accounting_periods WHERE organization_id=$1 AND month=$2", org, start).Scan(&existingStatus)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if existingStatus == "CLOSED" {
			return nil, ErrState
		}
		blockers, err := periodBlockers(c.Request.Context(), tx, org, start, end)
		if err != nil {
			return nil, err
		}
		if blockers > 0 {
			return nil, ErrState.WithMetadata(map[string]string{"field": "period_blockers", "count": fmt.Sprint(blockers)})
		}
		opening, covered, err := positionsAt(c.Request.Context(), tx, org, start)
		if err != nil {
			return nil, err
		}
		closing, _, err := positionsAt(c.Request.Context(), tx, org, end)
		if err != nil {
			return nil, err
		}
		if closing == nil {
			return nil, invalid("period_before_coverage")
		}
		if opening == nil && (!in.AcceptPartial || safeText(in.Reason) == "") {
			return nil, invalid("opening_balance_unknown")
		}
		snapshot, public, err := h.periodSnapshot(c.Request.Context(), tx, org, start, end, covered, opening, closing)
		if err != nil {
			return nil, err
		}
		id := uuid.NewString()
		var revision int64
		err = tx.QueryRowContext(c.Request.Context(), `INSERT INTO accounting_periods(id,organization_id,month,status,revision,reason) VALUES($1,$2,$3,'CLOSED',1,$4) ON CONFLICT(organization_id,month) DO UPDATE SET status='CLOSED',revision=accounting_periods.revision+1,reason=EXCLUDED.reason,updated_at=NOW() RETURNING id,revision`, id, org, start, in.Reason).Scan(&id, &revision)
		if err != nil {
			return nil, err
		}
		statementID := uuid.NewString()
		_, err = tx.ExecContext(c.Request.Context(), `INSERT INTO accounting_statements(id,organization_id,period_id,revision,snapshot,public_snapshot,created_by) VALUES($1,$2,$3,$4,$5,$6,$7)`, statementID, org, id, revision, snapshot, public, actor(c))
		return gin.H{"id": id, "statement_id": statementID, "revision": revision, "status": "CLOSED"}, err
	})
}
func periodBlockers(ctx context.Context, q dbQuery, org int64, start, end time.Time) (int64, error) {
	var count int64
	query := `WITH objects AS (` + recordUnion() + `) SELECT
 (SELECT COUNT(*) FROM objects WHERE organization_id=$1 AND occurred_at>=$2 AND occurred_at<$3 AND status='DRAFT')+
 (SELECT COUNT(*) FROM objects r WHERE r.organization_id=$1 AND r.occurred_at>=$2 AND r.occurred_at<$3 AND ((r.kind='transfers' AND r.status='RECEIVED') OR (r.kind='vcc-transactions' AND r.status='CONFIRMED') OR (r.kind='expenses' AND r.status IN ('UNPAID','PENDING','PAID'))) AND COALESCE((SELECT x.status FROM accounting_reconciliation x WHERE x.object_kind=r.kind AND x.object_id=r.id ORDER BY x.created_at DESC,x.id DESC LIMIT 1),'UNMATCHED') NOT IN ('MATCHED','EXCLUDED'))+
 (SELECT COUNT(*) FROM accounting_external_actions WHERE organization_id=$1 AND occurred_at>=$2 AND occurred_at<$3 AND status='PENDING')+
 (SELECT COUNT(*) FROM accounting_vendor_invoices i WHERE i.organization_id=$1 AND i.occurred_at>=$2 AND i.occurred_at<$3 AND i.status='CONFIRMED' AND i.amount<>COALESCE((SELECT SUM(a.amount) FROM accounting_cost_allocations a WHERE a.invoice_id=i.id AND a.status='ACTIVE'),0))`
	err := q.QueryRowContext(ctx, query, org, start, end).Scan(&count)
	return count, err
}
func (h *Handler) reopenPeriod(c *gin.Context) {
	if !requireAdmin(c) || !idValid(c, c.Param("id")) {
		return
	}
	in, ok := workflowDecode(c)
	if !ok {
		return
	}
	if safeText(in.Reason) == "" {
		response.ErrorFrom(c, invalid("reason"))
		return
	}
	var org int64
	var month time.Time
	err := h.engine.db.QueryRowContext(c.Request.Context(), "SELECT organization_id,month FROM accounting_periods WHERE id=$1", c.Param("id")).Scan(&org, &month)
	if response.ErrorFrom(c, err) {
		return
	}
	h.run(c, Operation{Org: org, Kind: "periods", Object: c.Param("id"), Action: "reopen", Payload: in}, func(tx *sql.Tx) (any, error) {
		var later bool
		if err := tx.QueryRowContext(c.Request.Context(), "SELECT EXISTS(SELECT 1 FROM accounting_periods WHERE organization_id=$1 AND month>$2 AND status='CLOSED')", org, month).Scan(&later); err != nil {
			return nil, err
		}
		if later {
			return nil, invalid("later_period_closed")
		}
		result, err := tx.ExecContext(c.Request.Context(), "UPDATE accounting_periods SET status='OPEN',reason=$2,updated_at=NOW() WHERE id=$1 AND status='CLOSED'", c.Param("id"), in.Reason)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n != 1 {
			return nil, ErrState
		}
		return gin.H{"id": c.Param("id"), "status": "OPEN", "reason": in.Reason}, nil
	})
}
func (h *Handler) periodSnapshot(ctx context.Context, q dbQuery, org int64, start, end, covered time.Time, opening, closing map[string]Position) ([]byte, []byte, error) {
	var publicID, name string
	if err := q.QueryRowContext(ctx, "SELECT public_id,name FROM desktop_organizations WHERE id=$1", org).Scan(&publicID, &name); err != nil {
		return nil, nil, err
	}
	rows, err := q.QueryContext(ctx, `WITH objects AS (`+recordUnion()+`) SELECT kind,id::text,status,currency,amount::text,occurred_at,details,customer_visible,created_at,version FROM objects WHERE organization_id=$1 AND occurred_at>=$2 AND occurred_at<$3 ORDER BY occurred_at,id`, org, start, end)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	records := []StatementEntry{}
	visible := []StatementEntry{}
	for rows.Next() {
		r := &Record{OrganizationID: publicID}
		var kind string
		var body []byte
		if err = rows.Scan(&kind, &r.ID, &r.Status, &r.Currency, &r.Amount, &r.OccurredAt, &body, &r.CustomerVisible, &r.CreatedAt, &r.Version); err != nil {
			return nil, nil, err
		}
		if err = json.Unmarshal(body, &r.Details); err != nil {
			return nil, nil, err
		}
		records = append(records, StatementEntry{Kind: kind, Record: r})
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, nil, err
	}
	for _, item := range records {
		r, kind := item.Record, item.Kind
		if kind == "transfers" || kind == "expenses" || kind == "receivables" {
			current, err := effective(ctx, q, kind, r)
			if err != nil {
				return nil, nil, err
			}
			r.Details["effective_amount"] = current.String()
			r.Details["original_amount"] = r.Amount
			r.Details["adjustment_amount"] = current.Sub(amount(r)).String()
			if kind == "transfers" {
				r.Details["received_amount"] = current.String()
			}
		}
		visibleReceipt := kind == "transfers" && !internalFunding(r)
		visibleTransaction := kind == "vcc-transactions" && (detail(r, "transaction_type") == "CONSUME" || detail(r, "transaction_type") == "REFUND")
		sharedDocument := r.CustomerVisible && (kind == "contracts" || kind == "invoices" && detail(r, "invoice_type") == "SALES")
		if (visibleReceipt || visibleTransaction || sharedDocument) && customerVisibleStatus(kind, r.Status) {
			visible = append(visible, StatementEntry{Kind: kind, Record: r.Public(kind)})
		}
	}
	movement := map[string]Position{}
	for currency, current := range closing {
		if opening == nil {
			continue
		}
		base := opening[currency]
		movement[currency] = Position{Cash: dec(current.Cash).Sub(dec(base.Cash)).String(), Card: dec(current.Card).Sub(dec(base.Card)).String()}
	}
	captured := time.Now().UTC()
	private := gin.H{"organization_id": publicID, "organization_name": name, "month": start.Format("2006-01"), "covered_from": covered, "opening": opening, "movement": movement, "closing": closing, "records": records, "captured_at": captured}
	public := gin.H{"organization_id": publicID, "organization_name": name, "month": start.Format("2006-01"), "covered_from": covered, "opening": opening, "movement": movement, "closing": closing, "records": visible, "captured_at": captured}
	a, err := json.Marshal(private)
	if err != nil {
		return nil, nil, err
	}
	b, err := json.Marshal(public)
	return a, b, err
}

func (h *Handler) statementBody(c *gin.Context) (json.RawMessage, bool) {
	if !idValid(c, c.Param("id")) {
		return nil, false
	}
	column := "snapshot"
	if orgScope(c) > 0 {
		column = "public_snapshot"
	}
	var raw []byte
	var org int64
	query := "SELECT s." + column + ",s.organization_id FROM accounting_statements s JOIN accounting_periods p ON p.id=s.period_id WHERE s.id=$1"
	args := []any{c.Param("id")}
	if orgScope(c) > 0 {
		query += " AND s.organization_id=$2 AND p.status='CLOSED' AND s.revision=p.revision"
		args = append(args, orgScope(c))
	}
	err := h.engine.db.QueryRowContext(c.Request.Context(), query, args...).Scan(&raw, &org)
	if err == sql.ErrNoRows {
		err = ErrNotFound
	}
	if err == nil {
		action := "read-statement"
		if strings.HasSuffix(c.FullPath(), "/export") {
			action = "export"
		}
		err = audit(c.Request.Context(), h.engine.db, org, "statements", c.Param("id"), action, actor(c), requestID(c), map[string]any{})
	}
	return json.RawMessage(raw), !response.ErrorFrom(c, err)
}
func (h *Handler) statement(c *gin.Context) {
	body, ok := h.statementBody(c)
	if ok {
		response.Success(c, body)
	}
}
func (h *Handler) statementExport(c *gin.Context) {
	body, ok := h.statementBody(c)
	if !ok {
		return
	}
	var snapshot struct {
		Month    string              `json:"month"`
		Opening  map[string]Position `json:"opening"`
		Movement map[string]Position `json:"movement"`
		Closing  map[string]Position `json:"closing"`
		Records  []StatementEntry    `json:"records"`
	}
	if response.ErrorFrom(c, json.Unmarshal(body, &snapshot)) {
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="statement-`+snapshot.Month+`.csv"`)
	_, _ = c.Writer.Write([]byte("\xef\xbb\xbf"))
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"month", "currency", "opening_cash", "opening_card", "cash_movement", "card_movement", "closing_cash", "closing_card"})
	for currency, p := range snapshot.Closing {
		o, m := snapshot.Opening[currency], snapshot.Movement[currency]
		_ = w.Write([]string{snapshot.Month, currency, o.Cash, o.Card, m.Cash, m.Card, p.Cash, p.Card})
	}
	_ = w.Write([]string{})
	_ = w.Write([]string{"kind", "id", "record", "currency", "effective_amount", "original_amount", "occurred_at", "status", "bank_reference", "invoice_number", "contract_number", "merchant", "last_four"})
	for _, entry := range snapshot.Records {
		r := entry.Record
		name := r.ID
		for _, field := range []string{"alias", "name", "payer_name", "merchant", "vendor", "supplier", "invoice_number"} {
			if value := detail(r, field); value != "" {
				name = value
				break
			}
		}
		effective := detail(r, "effective_amount")
		if effective == "" {
			effective = r.Amount
		}
		_ = w.Write([]string{entry.Kind, r.ID, csvText(name), r.Currency, effective, r.Amount, r.OccurredAt.Format(time.RFC3339), r.Status, csvText(detail(r, "bank_reference")), csvText(detail(r, "invoice_number")), csvText(detail(r, "contract_number")), csvText(detail(r, "merchant")), detail(r, "last_four")})
	}
	w.Flush()
}
