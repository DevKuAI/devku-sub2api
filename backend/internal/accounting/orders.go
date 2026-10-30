package accounting

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Orders group one customer deal: the receipt, the sales invoice, the card purchases and the AI vendor invoices.
var orderKinds = map[string]bool{"transfers": true, "invoices": true, "vcc-cards": true, "vcc-transactions": true, "vendor-invoices": true}

// Records that may join an order after they were created elsewhere; cards and card spend are only created inside one.
var orderLinkKinds = map[string]bool{"transfers": true, "invoices": true, "vendor-invoices": true}

// internalFunding marks transfers the system creates (FX target funds, card top-ups) rather than customer payments.
func internalFunding(r *Record) bool { return detail(r, "funding_type") != "" }

type OrderProgress struct {
	Receipt   bool `json:"receipt"`
	Invoice   bool `json:"invoice"`
	Purchase  bool `json:"purchase"`
	AIInvoice bool `json:"ai_invoice"`
}

type OrderTotal struct {
	Currency string `json:"currency"`
	Received string `json:"received"`
	Invoiced string `json:"invoiced"`
	Spent    string `json:"spent"`
	AICost   string `json:"ai_cost"`
}

type Order struct {
	ID             string               `json:"id"`
	OrganizationID string               `json:"organization_id"`
	Organization   string               `json:"organization_name"`
	Name           string               `json:"name"`
	Remark         string               `json:"remark"`
	Status         string               `json:"status"`
	State          string               `json:"state"`
	CloseReason    string               `json:"close_reason"`
	ClosedAt       *time.Time           `json:"closed_at,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	Version        int64                `json:"version"`
	Progress       OrderProgress        `json:"progress"`
	Totals         []OrderTotal         `json:"totals"`
	Items          map[string][]*Record `json:"items,omitempty"`
	Org            int64                `json:"-"`
}

type orderInput struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Remark         string `json:"remark"`
	Version        int64  `json:"version"`
	Reason         string `json:"reason"`
	ObjectKind     string `json:"object_kind"`
	ObjectID       string `json:"object_id"`
	Transfer       *Input `json:"transfer,omitempty"`
}

type topUpInput struct {
	Amount     string `json:"amount"`
	OccurredAt string `json:"occurred_at"`
	Reference  string `json:"reference"`
	Remark     string `json:"remark"`
}

func (h *Handler) registerOrders(g *gin.RouterGroup) {
	g.GET("/orders", h.listOrders)
	g.POST("/orders", h.createOrder)
	g.GET("/orders/:id", h.getOrder)
	g.PATCH("/orders/:id", h.updateOrder)
	g.POST("/orders/:id/links", h.linkOrder)
	g.POST("/orders/:id/close", h.closeOrder)
	g.POST("/orders/:id/reopen", h.reopenOrder)
	g.POST("/vcc-cards/:id/top-ups", h.topUpCard)
}

// --- Order membership --------------------------------------------------------

func orderOf(ctx context.Context, q dbQuery, kind, id string) (string, error) {
	var order string
	err := q.QueryRowContext(ctx, "SELECT order_id::text FROM accounting_order_items WHERE object_kind=$1 AND object_id=$2", kind, id).Scan(&order)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return order, err
}

func lockOrder(ctx context.Context, q dbQuery, id string, org int64, mustBeOpen bool) error {
	if _, err := uuid.Parse(id); err != nil {
		return invalid("order_id")
	}
	var status string
	err := q.QueryRowContext(ctx, "SELECT status FROM accounting_orders WHERE id=$1 AND organization_id=$2 FOR UPDATE", id, org).Scan(&status)
	if err == sql.ErrNoRows {
		return invalid("order_id")
	}
	if err != nil {
		return err
	}
	if mustBeOpen && status != "OPEN" {
		return ErrState.WithMetadata(map[string]string{"field": "order_closed"})
	}
	return nil
}

func linkToOrder(ctx context.Context, q dbQuery, order, kind string, r *Record, actorID int64) error {
	if kind == "invoices" && detail(r, "invoice_type") != "SALES" {
		return invalid("order_id")
	}
	if kind == "transfers" && internalFunding(r) {
		return invalid("order_id")
	}
	_, err := q.ExecContext(ctx, "INSERT INTO accounting_order_items(object_kind,object_id,order_id,organization_id,created_by) VALUES($1,$2,$3,$4,$5)", kind, r.ID, order, r.Org, actorID)
	return err
}

// attachToOrder places a new record in its order. Cards and card spend must belong to one; refunds and
// credits follow the order of the purchase they correct, even after that order was closed.
func attachToOrder(ctx context.Context, tx dbQuery, kind string, r *Record, order string, actorID int64) error {
	if !orderKinds[kind] {
		if order != "" {
			return invalid("order_id")
		}
		return nil
	}
	inherited := false
	if order == "" && kind == "vcc-transactions" && detail(r, "original_transaction_id") != "" {
		var err error
		if order, err = orderOf(ctx, tx, "vcc-transactions", detail(r, "original_transaction_id")); err != nil {
			return err
		}
		inherited = order != ""
	}
	if order == "" {
		if kind == "vcc-cards" || kind == "vcc-transactions" {
			return invalid("order_id")
		}
		return nil
	}
	if err := lockOrder(ctx, tx, order, r.Org, !inherited); err != nil {
		return err
	}
	return linkToOrder(ctx, tx, order, kind, r, actorID)
}

func orderRecords(ctx context.Context, q dbQuery, order, kind string, lock bool) ([]*Record, error) {
	query := "SELECT " + recordColumns + " FROM " + specs[kind].table + " r JOIN desktop_organizations o ON o.id=r.organization_id JOIN accounting_order_items i ON i.object_kind=$2 AND i.object_id=r.id WHERE i.order_id=$1"
	if kind == "vcc-cards" {
		// Cards created in the order plus any card its spend was charged to.
		query = "SELECT " + recordColumns + " FROM accounting_vcc_cards r JOIN desktop_organizations o ON o.id=r.organization_id WHERE r.id IN (SELECT object_id FROM accounting_order_items WHERE order_id=$1 AND object_kind=$2 UNION SELECT t.parent_id FROM accounting_vcc_transactions t JOIN accounting_order_items i ON i.object_kind='vcc-transactions' AND i.object_id=t.id WHERE i.order_id=$1)"
	}
	query += " ORDER BY r.occurred_at,r.id"
	if lock {
		query += " FOR UPDATE OF r"
	}
	rows, err := q.QueryContext(ctx, query, order, kind)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*Record{}
	for rows.Next() {
		r, scanErr := scanRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Automatic funding and cost allocation -----------------------------------

// fundFromOrder covers card spend that no allocation funds yet, using the order's own customer receipts in
// the card currency. Cross-currency spend stays uncovered until the card is topped up.
func (e *Engine) fundFromOrder(ctx context.Context, tx *sql.Tx, order string, card *Record, actorID int64) error {
	if card.Status != "ACTIVE" {
		return nil
	}
	uncovered, err := sum(ctx, tx, `SELECT COALESCE(SUM(f.amount-f.refunded_amount),0)::text FROM accounting_funding f JOIN accounting_vcc_transactions t ON t.id=f.transaction_id
 WHERE t.parent_id=$1 AND t.status='CONFIRMED' AND f.allocation_id IS NULL AND f.source_funding_id IS NULL`, card.ID)
	if err != nil || !uncovered.IsPositive() {
		return err
	}
	receipts, err := orderRecords(ctx, tx, order, "transfers", true)
	if err != nil {
		return err
	}
	for _, receipt := range receipts {
		if !uncovered.IsPositive() {
			break
		}
		if receipt.Status != "RECEIVED" || receipt.Currency != card.Currency || internalFunding(receipt) {
			continue
		}
		available, availableErr := availableTransfer(ctx, tx, receipt)
		if availableErr != nil {
			return availableErr
		}
		part := decimal.Min(uncovered, available)
		if !part.IsPositive() {
			continue
		}
		if _, err = e.allocate(ctx, tx, receipt, card.ID, part.String(), actorID, false); err != nil {
			return err
		}
		uncovered = uncovered.Sub(part)
	}
	return nil
}

// allocateOrderCosts matches the order's confirmed AI invoices to its confirmed card purchases in the same
// currency, oldest first, within the existing cost limits.
func (e *Engine) allocateOrderCosts(ctx context.Context, tx *sql.Tx, order string, org, actorID int64) error {
	invoices, err := orderRecords(ctx, tx, order, "vendor-invoices", false)
	if err != nil {
		return err
	}
	spends, err := orderRecords(ctx, tx, order, "vcc-transactions", false)
	if err != nil {
		return err
	}
	for _, invoice := range invoices {
		if invoice.Status != "CONFIRMED" {
			continue
		}
		used, sumErr := sum(ctx, tx, "SELECT COALESCE(SUM(amount),0)::text FROM accounting_cost_allocations WHERE invoice_id=$1 AND status='ACTIVE'", invoice.ID)
		if sumErr != nil {
			return sumErr
		}
		remaining := amount(invoice).Sub(used)
		for _, spend := range spends {
			if !remaining.IsPositive() {
				break
			}
			if spend.Status != "CONFIRMED" || detail(spend, "transaction_type") != "CONSUME" || spend.Currency != invoice.Currency {
				continue
			}
			free, freeErr := unallocatedCost(ctx, tx, org, spend)
			if freeErr != nil {
				return freeErr
			}
			part := decimal.Min(remaining, free)
			if !part.IsPositive() {
				continue
			}
			in := workflowInput{InvoiceID: invoice.ID, SourceKind: "vcc-transactions", SourceID: spend.ID, Amount: part.String()}
			if _, err = e.allocateCost(ctx, tx, org, in, actorID); err != nil {
				return err
			}
			remaining = remaining.Sub(part)
		}
	}
	return nil
}

func unallocatedCost(ctx context.Context, q dbQuery, org int64, spend *Record) (decimal.Decimal, error) {
	identity, limit, err := costSource(ctx, q, org, "vcc-transactions", spend)
	if err != nil {
		return decimal.Zero, err
	}
	occupied, err := sum(ctx, q, "SELECT COALESCE(SUM(source_amount),0)::text FROM accounting_cost_allocations WHERE cost_identity=$1 AND status='ACTIVE'", identity)
	return limit.Sub(occupied), err
}

// afterOrderChange keeps an order's funding and costs in step when one of its records is confirmed or linked.
func (e *Engine) afterOrderChange(ctx context.Context, tx *sql.Tx, kind string, r *Record, actorID int64) error {
	order, err := orderOf(ctx, tx, kind, r.ID)
	if err != nil || order == "" {
		return err
	}
	switch kind {
	case "vcc-transactions":
		if !isCredit(r) {
			card, cardErr := get(ctx, tx, "vcc-cards", r.ParentID, r.Org, true)
			if cardErr != nil {
				return cardErr
			}
			if err = e.fundFromOrder(ctx, tx, order, card, actorID); err != nil {
				return err
			}
		}
	case "transfers":
		if r.Status != "RECEIVED" {
			return nil
		}
		cards, cardErr := orderRecords(ctx, tx, order, "vcc-cards", true)
		if cardErr != nil {
			return cardErr
		}
		for _, card := range cards {
			if err = e.fundFromOrder(ctx, tx, order, card, actorID); err != nil {
				return err
			}
		}
		return nil
	case "vendor-invoices":
		if r.Status != "CONFIRMED" {
			return nil
		}
	default:
		return nil
	}
	return e.allocateOrderCosts(ctx, tx, order, r.Org, actorID)
}

// --- Reading orders -----------------------------------------------------------

const orderColumns = "a.id::text,o.public_id,o.name,a.name,a.remark,a.status,a.close_reason,a.closed_at,a.created_at,a.version,a.organization_id"

func scanOrder(row interface{ Scan(...any) error }) (*Order, error) {
	o := &Order{}
	err := row.Scan(&o.ID, &o.OrganizationID, &o.Organization, &o.Name, &o.Remark, &o.Status, &o.CloseReason, &o.ClosedAt, &o.CreatedAt, &o.Version, &o.Org)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return o, err
}

func (e *Engine) loadOrder(ctx context.Context, q dbQuery, id string, org int64, withItems bool) (*Order, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	query := "SELECT " + orderColumns + " FROM accounting_orders a JOIN desktop_organizations o ON o.id=a.organization_id WHERE a.id=$1 AND o.deleted_at IS NULL AND ($2::bigint=0 OR a.organization_id=$2)"
	o, err := scanOrder(q.QueryRowContext(ctx, query, id, org))
	if err != nil {
		return nil, err
	}
	return o, e.summarizeOrder(ctx, q, o, withItems)
}

func (e *Engine) summarizeOrder(ctx context.Context, q dbQuery, o *Order, withItems bool) error {
	items := map[string][]*Record{}
	for kind := range orderKinds {
		records, err := orderRecords(ctx, q, o.ID, kind, false)
		if err != nil {
			return err
		}
		for _, r := range records {
			if err = e.enrich(ctx, kind, r); err != nil {
				return err
			}
		}
		items[kind] = records
	}
	totals := map[string]*struct{ received, invoiced, spent, aiCost decimal.Decimal }{}
	total := func(currency string) *struct{ received, invoiced, spent, aiCost decimal.Decimal } {
		if totals[currency] == nil {
			totals[currency] = &struct{ received, invoiced, spent, aiCost decimal.Decimal }{}
		}
		return totals[currency]
	}
	for _, r := range items["transfers"] {
		if r.Status == "RECEIVED" {
			o.Progress.Receipt = true
			total(r.Currency).received = total(r.Currency).received.Add(dec(detail(r, "effective_amount")))
		}
	}
	for _, r := range items["invoices"] {
		if r.Status == "VERIFIED" {
			o.Progress.Invoice = true
			total(r.Currency).invoiced = total(r.Currency).invoiced.Add(amount(r))
		}
	}
	allCostsCovered := true
	for _, r := range items["vcc-transactions"] {
		if r.Status != "CONFIRMED" {
			continue
		}
		if isCredit(r) {
			total(r.Currency).spent = total(r.Currency).spent.Sub(amount(r))
			continue
		}
		total(r.Currency).spent = total(r.Currency).spent.Add(amount(r))
		if detail(r, "transaction_type") != "CONSUME" {
			continue
		}
		o.Progress.Purchase = true
		free, err := unallocatedCost(ctx, q, o.Org, r)
		if err != nil {
			return err
		}
		if free.IsPositive() {
			allCostsCovered = false
		}
	}
	for _, r := range items["vendor-invoices"] {
		if r.Status == "CONFIRMED" {
			o.Progress.AIInvoice = true
			total(r.Currency).aiCost = total(r.Currency).aiCost.Add(amount(r))
		}
	}
	o.Progress.AIInvoice = o.Progress.AIInvoice && allCostsCovered
	switch {
	case o.Status == "CLOSED":
		o.State = "CLOSED"
	case o.Progress.Receipt && o.Progress.Invoice && o.Progress.Purchase && o.Progress.AIInvoice:
		o.State = "COMPLETED"
	default:
		o.State = "OPEN"
	}
	currencies := make([]string, 0, len(totals))
	for currency := range totals {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	o.Totals = []OrderTotal{}
	for _, currency := range currencies {
		v := totals[currency]
		o.Totals = append(o.Totals, OrderTotal{Currency: currency, Received: v.received.String(), Invoiced: v.invoiced.String(), Spent: v.spent.String(), AICost: v.aiCost.String()})
	}
	if withItems {
		if err := countAttachments(ctx, q, items); err != nil {
			return err
		}
		o.Items = items
	}
	return nil
}

// countAttachments notes how many completed files each entry has, so the order page can flag missing proof.
func countAttachments(ctx context.Context, q dbQuery, items map[string][]*Record) error {
	byID := map[string]*Record{}
	ids := []string{}
	for _, records := range items {
		for _, r := range records {
			byID[r.ID] = r
			ids = append(ids, r.ID)
			r.Details["attachment_count"] = "0"
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.QueryContext(ctx, "SELECT object_id::text,COUNT(*)::text FROM accounting_attachments WHERE object_id::text=ANY($1) AND status='COMPLETE' AND deleted_at IS NULL GROUP BY object_id", pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, count string
		if err = rows.Scan(&id, &count); err != nil {
			return err
		}
		byID[id].Details["attachment_count"] = count
	}
	return rows.Err()
}

// --- HTTP handlers ------------------------------------------------------------

func orderDecode(c *gin.Context) (orderInput, bool) {
	var in orderInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return in, false
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Remark = strings.TrimSpace(in.Remark)
	in.Reason = strings.TrimSpace(in.Reason)
	return in, true
}

func validOrderText(in orderInput) error {
	if in.Name == "" || len([]rune(in.Name)) > 200 {
		return invalid("name")
	}
	if len(in.Remark) > 4000 {
		return invalid("remark")
	}
	return nil
}

func (h *Handler) orderRecord(c *gin.Context) *Order {
	o, err := h.engine.loadOrder(c.Request.Context(), h.engine.db, c.Param("id"), orgScope(c), false)
	if response.ErrorFrom(c, err) {
		return nil
	}
	return o
}

// listOrders lists one organization's orders, or every organization's when none is given.
func (h *Handler) listOrders(c *gin.Context) {
	org := orgScope(c)
	if org == 0 && c.Query("organization_id") != "" {
		var ok bool
		if org, ok = h.workflowOrg(c, c.Query("organization_id")); !ok {
			return
		}
	}
	search := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.TrimSpace(c.Query("search"))) + "%"
	ctx := c.Request.Context()
	rows, err := h.engine.db.QueryContext(ctx, "SELECT "+orderColumns+" FROM accounting_orders a JOIN desktop_organizations o ON o.id=a.organization_id WHERE o.deleted_at IS NULL AND ($1::bigint=0 OR a.organization_id=$1) AND (a.name ILIKE $2 OR o.name ILIKE $2) ORDER BY a.created_at DESC,a.id LIMIT 500", org, search)
	if response.ErrorFrom(c, err) {
		return
	}
	orders := []*Order{}
	for rows.Next() {
		o, scanErr := scanOrder(rows)
		if response.ErrorFrom(c, scanErr) {
			_ = rows.Close()
			return
		}
		orders = append(orders, o)
	}
	_ = rows.Close()
	if response.ErrorFrom(c, rows.Err()) {
		return
	}
	// Progress is derived from the linked records, so the state filter runs after summarizing.
	state := strings.ToUpper(c.Query("state"))
	filtered := []*Order{}
	for _, o := range orders {
		if response.ErrorFrom(c, h.engine.summarizeOrder(ctx, h.engine.db, o, false)) {
			return
		}
		if state == "" || o.State == state {
			filtered = append(filtered, o)
		}
	}
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("page_size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	start := min((page-1)*size, len(filtered))
	end := min(start+size, len(filtered))
	response.Paginated(c, filtered[start:end], int64(len(filtered)), page, size)
}

func (h *Handler) getOrder(c *gin.Context) {
	o, err := h.engine.loadOrder(c.Request.Context(), h.engine.db, c.Param("id"), orgScope(c), true)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, o)
}

func (h *Handler) createOrder(c *gin.Context) {
	in, ok := orderDecode(c)
	if !ok {
		return
	}
	if response.ErrorFrom(c, validOrderText(in)) {
		return
	}
	org, ok := h.workflowOrg(c, in.OrganizationID)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: org, Kind: "orders", Action: "create", Payload: in}, func(tx *sql.Tx) (any, error) {
		id := uuid.NewString()
		if _, err := tx.ExecContext(ctx, "INSERT INTO accounting_orders(id,organization_id,name,remark,created_by) VALUES($1,$2,$3,$4,$5)", id, org, in.Name, in.Remark, actor(c)); err != nil {
			return nil, err
		}
		// The first receipt is usually known when the order starts, so it can be recorded in the same step.
		if in.Transfer != nil {
			receipt := *in.Transfer
			receipt.OrganizationID = in.OrganizationID
			receipt.OrderID = id
			r, err := h.engine.Create(ctx, tx, "transfers", receipt, org, actor(c))
			if err != nil {
				return nil, err
			}
			if err = checkOpenPeriod(ctx, tx, org, r.OccurredAt); err != nil {
				return nil, err
			}
			if err = audit(ctx, tx, org, "transfers", r.ID, "create", actor(c), requestID(c), map[string]any{"order_id": id, "amount": r.Amount, "currency": r.Currency}); err != nil {
				return nil, err
			}
		}
		return h.engine.loadOrder(ctx, tx, id, org, false)
	})
}

func (h *Handler) updateOrder(c *gin.Context) {
	current := h.orderRecord(c)
	if current == nil {
		return
	}
	in, ok := orderDecode(c)
	if !ok || response.ErrorFrom(c, validOrderText(in)) {
		return
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: current.Org, Kind: "orders", Object: current.ID, Action: "update", Payload: in}, func(tx *sql.Tx) (any, error) {
		result, err := tx.ExecContext(ctx, "UPDATE accounting_orders SET name=$3,remark=$4,updated_at=NOW(),version=version+1 WHERE id=$1 AND version=$2", current.ID, in.Version, in.Name, in.Remark)
		if err != nil {
			return nil, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return nil, ErrState
		}
		return h.engine.loadOrder(ctx, tx, current.ID, current.Org, false)
	})
}

func (h *Handler) linkOrder(c *gin.Context) {
	current := h.orderRecord(c)
	if current == nil {
		return
	}
	in, ok := orderDecode(c)
	if !ok {
		return
	}
	if !orderLinkKinds[in.ObjectKind] {
		response.ErrorFrom(c, invalid("object_kind"))
		return
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: current.Org, Kind: "orders", Object: current.ID, Action: "link", Payload: in}, func(tx *sql.Tx) (any, error) {
		if err := lockOrder(ctx, tx, current.ID, current.Org, true); err != nil {
			return nil, err
		}
		r, err := get(ctx, tx, in.ObjectKind, in.ObjectID, current.Org, true)
		if err != nil {
			return nil, err
		}
		if r.Status == "VOID" {
			return nil, ErrState
		}
		if err = linkToOrder(ctx, tx, current.ID, in.ObjectKind, r, actor(c)); err != nil {
			return nil, err
		}
		if err = h.engine.afterOrderChange(ctx, tx, in.ObjectKind, r, actor(c)); err != nil {
			return nil, err
		}
		return h.engine.loadOrder(ctx, tx, current.ID, current.Org, false)
	})
}

func (h *Handler) closeOrder(c *gin.Context) {
	current := h.orderRecord(c)
	if current == nil {
		return
	}
	in, ok := orderDecode(c)
	if !ok {
		return
	}
	if in.Reason == "" || len(in.Reason) > 4000 {
		response.ErrorFrom(c, invalid("reason"))
		return
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: current.Org, Kind: "orders", Object: current.ID, Action: "close", Payload: in}, func(tx *sql.Tx) (any, error) {
		result, err := tx.ExecContext(ctx, "UPDATE accounting_orders SET status='CLOSED',close_reason=$2,closed_at=NOW(),closed_by=$3,updated_at=NOW(),version=version+1 WHERE id=$1 AND status='OPEN'", current.ID, in.Reason, actor(c))
		if err != nil {
			return nil, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return nil, ErrState
		}
		return h.engine.loadOrder(ctx, tx, current.ID, current.Org, false)
	})
}

func (h *Handler) reopenOrder(c *gin.Context) {
	current := h.orderRecord(c)
	if current == nil {
		return
	}
	in, ok := orderDecode(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: current.Org, Kind: "orders", Object: current.ID, Action: "reopen", Payload: in}, func(tx *sql.Tx) (any, error) {
		result, err := tx.ExecContext(ctx, "UPDATE accounting_orders SET status='OPEN',close_reason='',closed_at=NULL,closed_by=NULL,updated_at=NOW(),version=version+1 WHERE id=$1 AND status='CLOSED'", current.ID)
		if err != nil {
			return nil, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return nil, ErrState
		}
		return h.engine.loadOrder(ctx, tx, current.ID, current.Org, false)
	})
}

// topUpCard records our own money loaded onto a card. Like FX target funds it is an internal funding source:
// not customer income, hidden from the organization, and already executed at the provider.
func (h *Handler) topUpCard(c *gin.Context) {
	card := h.record(c, "vcc-cards")
	if card == nil {
		return
	}
	var in topUpInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	in.Reference = strings.TrimSpace(in.Reference)
	in.Remark = strings.TrimSpace(in.Remark)
	if len(in.Reference) > 200 || len(in.Remark) > 4000 {
		response.ErrorFrom(c, ErrInvalid)
		return
	}
	ctx := c.Request.Context()
	h.run(c, Operation{Org: card.Org, Kind: "vcc-cards", Object: card.ID, Action: "top-up", Payload: in}, func(tx *sql.Tx) (any, error) {
		current, err := get(ctx, tx, "vcc-cards", card.ID, card.Org, true)
		if err != nil {
			return nil, err
		}
		if current.Status != "ACTIVE" {
			return nil, ErrState
		}
		value, err := number(in.Amount, current.Currency, true)
		if err != nil {
			return nil, err
		}
		occurred, err := workflowTime(in.OccurredAt)
		if err != nil {
			return nil, err
		}
		id := uuid.NewString()
		funding := &Record{ID: id, Org: current.Org, OrganizationID: current.OrganizationID, Status: "RECEIVED", Currency: current.Currency, Amount: value.String(), OccurredAt: occurred, Version: 1, Details: map[string]any{
			"funding_type": "TOPUP", "card_id": current.ID, "payer_name": "Card top-up", "bank_reference": "TOPUP-" + id,
			"reference": in.Reference, "remark": in.Remark, "sales_amount": "0", "received_amount": value.String(),
		}}
		if err = insertRecord(ctx, tx, "transfers", funding, actor(c)); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO accounting_reconciliation(id,organization_id,object_kind,object_id,status,reference,reference_amount,currency,reason,created_by) VALUES($1,$2,'transfers',$3,'MATCHED',$4,$5,$6,'Card top-up',$7)`, uuid.NewString(), current.Org, id, in.Reference, funding.Amount, funding.Currency, actor(c)); err != nil {
			return nil, err
		}
		allocation, err := h.engine.allocate(ctx, tx, funding, current.ID, value.String(), actor(c), false)
		if err != nil {
			return nil, err
		}
		return gin.H{"id": id, "allocation_id": allocation.ID, "card_id": current.ID, "amount": value.String(), "currency": current.Currency, "reference": in.Reference}, nil
	})
}
