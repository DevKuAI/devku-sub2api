package accounting

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	VendorCardProvider = "CARD_PROVIDER"
	VendorAI           = "AI_VENDOR"
)

// Vendor is an entry of a maintained list: card providers issue our cards, AI vendors are who we buy from.
type Vendor struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Aliases   []string  `json:"aliases"`
	Status    string    `json:"status"`
	Sort      int       `json:"sort"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type VendorInput struct {
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	Status  string   `json:"status"`
	Sort    int      `json:"sort"`
	Remark  string   `json:"remark"`
}

// vendorFields maps each record kind to the detail that holds the vendor ID, the name snapshot kept beside it
// and the list it must come from.
var vendorFields = map[string]struct{ id, name, kind string }{
	"vcc-cards":        {"provider_id", "provider", VendorCardProvider},
	"vcc-transactions": {"vendor_id", "merchant", VendorAI},
	"vendor-invoices":  {"vendor_id", "vendor", VendorAI},
}

const vendorColumns = "id::text,kind,name,aliases,status,sort,remark,created_at,updated_at"

func scanVendor(row interface{ Scan(...any) error }) (*Vendor, error) {
	v := &Vendor{}
	var aliases []byte
	err := row.Scan(&v.ID, &v.Kind, &v.Name, &aliases, &v.Status, &v.Sort, &v.Remark, &v.CreatedAt, &v.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(aliases, &v.Aliases)
	return v, err
}

func getVendor(ctx context.Context, q dbQuery, id string, lock bool) (*Vendor, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrNotFound
	}
	query := "SELECT " + vendorColumns + " FROM accounting_vendors WHERE id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	return scanVendor(q.QueryRowContext(ctx, query, id))
}

func (e *Engine) ListVendors(ctx context.Context, kind string) ([]Vendor, error) {
	if kind != "" && kind != VendorCardProvider && kind != VendorAI {
		return nil, invalid("kind")
	}
	rows, err := e.db.QueryContext(ctx, "SELECT "+vendorColumns+" FROM accounting_vendors WHERE ($1='' OR kind=$1) ORDER BY kind,status,sort,lower(name)", kind)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Vendor{}
	for rows.Next() {
		v, scanErr := scanVendor(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func normalizeVendor(in *VendorInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 100 {
		return invalid("name")
	}
	if in.Status == "" {
		in.Status = "ACTIVE"
	}
	if in.Status != "ACTIVE" && in.Status != "DISABLED" {
		return invalid("status")
	}
	seen := map[string]bool{}
	aliases := []string{}
	for _, alias := range append([]string{in.Name}, in.Aliases...) {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" || seen[alias] {
			continue
		}
		// Very short fragments would match unrelated merchants.
		if len([]rune(alias)) < 3 || len([]rune(alias)) > 64 {
			return invalid("aliases")
		}
		seen[alias] = true
		aliases = append(aliases, alias)
	}
	if len(aliases) > 20 {
		return invalid("aliases")
	}
	in.Aliases = aliases
	in.Remark = strings.TrimSpace(in.Remark)
	if len(in.Remark) > 1000 {
		return invalid("remark")
	}
	return nil
}

func (e *Engine) CreateVendor(ctx context.Context, tx *sql.Tx, in VendorInput, actor int64) (*Vendor, error) {
	if in.Kind != VendorCardProvider && in.Kind != VendorAI {
		return nil, invalid("kind")
	}
	if err := normalizeVendor(&in); err != nil {
		return nil, err
	}
	aliases, _ := json.Marshal(in.Aliases)
	id := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `INSERT INTO accounting_vendors(id,kind,name,aliases,status,sort,remark,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, in.Kind, in.Name, aliases, in.Status, in.Sort, in.Remark, actor); err != nil {
		return nil, err
	}
	if err := classifyVendors(ctx, tx); err != nil {
		return nil, err
	}
	return getVendor(ctx, tx, id, false)
}

// UpdateVendor renames, re-aliases, reorders or disables a vendor; its kind is fixed. Records keep the name
// they were saved with.
func (e *Engine) UpdateVendor(ctx context.Context, tx *sql.Tx, id string, in VendorInput) (*Vendor, error) {
	current, err := getVendor(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if in.Kind != "" && in.Kind != current.Kind {
		return nil, invalid("kind")
	}
	if err = normalizeVendor(&in); err != nil {
		return nil, err
	}
	aliases, _ := json.Marshal(in.Aliases)
	if _, err = tx.ExecContext(ctx, `UPDATE accounting_vendors SET name=$2,aliases=$3,status=$4,sort=$5,remark=$6,updated_at=NOW() WHERE id=$1`,
		id, in.Name, aliases, in.Status, in.Sort, in.Remark); err != nil {
		return nil, err
	}
	if err = classifyVendors(ctx, tx); err != nil {
		return nil, err
	}
	return getVendor(ctx, tx, id, false)
}

// applyVendor checks the vendor a record refers to and stores the vendor's current name beside the ID. A
// disabled vendor may stay on a record that already had it but cannot be chosen anew.
func applyVendor(ctx context.Context, q dbQuery, kind string, r, old *Record) error {
	field, ok := vendorFields[kind]
	if !ok {
		return nil
	}
	id := detail(r, field.id)
	if kind == "vcc-transactions" && detail(r, "transaction_type") != "CONSUME" && id == "" {
		// Refunds, fees and adjustments may name a vendor; only spend must.
		return nil
	}
	if id == "" {
		return invalid(field.id)
	}
	v, err := getVendor(ctx, q, id, false)
	if err == ErrNotFound || (err == nil && v.Kind != field.kind) {
		return invalid(field.id)
	}
	if err != nil {
		return err
	}
	unchanged := old != nil && detail(old, field.id) == id
	if v.Status != "ACTIVE" && !unchanged {
		return invalid(field.id)
	}
	r.Details[field.name] = v.Name
	if unchanged && detail(old, field.name) != "" {
		// The name saved with the record stays as it was, even if the vendor was renamed since.
		r.Details[field.name] = detail(old, field.name)
	}
	return nil
}

// classifyVendors links records saved before vendor lists existed to the vendor whose name or alias appears in
// their free text. Records that match no vendor, or more than one, stay unclassified until someone picks one.
func classifyVendors(ctx context.Context, q dbQuery) error {
	for kind, field := range vendorFields {
		table := specs[kind].table
		_, err := q.ExecContext(ctx, `UPDATE `+table+` r SET details = r.details || jsonb_build_object('`+field.id+`', m.vendor_id)
 FROM (
   SELECT c.id, MIN(v.id::text) AS vendor_id, COUNT(DISTINCT v.id) AS matches
   FROM `+table+` c JOIN accounting_vendors v ON v.kind=$1 AND v.status='ACTIVE'
   WHERE COALESCE(c.details->>'`+field.id+`','')='' AND COALESCE(c.details->>'`+field.name+`','')<>''
     AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(v.aliases) a WHERE strpos(lower(c.details->>'`+field.name+`'), a) > 0)
   GROUP BY c.id
 ) m
 WHERE r.id=m.id AND m.matches=1`, field.kind)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) classifyVendorsAtStart(ctx context.Context) {
	if err := classifyVendors(ctx, e.db); err != nil {
		logger.LegacyPrintf("service.accounting", "Warning: vendor classification skipped: %v", err)
	}
}
