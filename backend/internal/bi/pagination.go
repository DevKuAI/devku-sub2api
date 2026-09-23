package bi

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
)

type pageCursor struct {
	Snapshot string `json:"snapshot"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

func (s *Service) encodeCursor(c pageCursor, userID int64, operation string) string {
	data, _ := json.Marshal(c)
	encoded := base64.RawURLEncoding.EncodeToString(data)
	return encoded + "." + keyedHash(s.identity, "cursor", jsonID(userID), operation, encoded)
}

func (s *Service) decodeCursor(raw string, userID int64, operation string, limit int) (pageCursor, error) {
	var cursor pageCursor
	parts := strings.Split(raw, ".")
	if len(raw) > 2048 || len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(keyedHash(s.identity, "cursor", jsonID(userID), operation, parts[0]))) {
		return cursor, invalid("cursor", "Cursor does not belong to this request")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Offset < 0 || cursor.Limit != limit || cursor.Snapshot == "" {
		return cursor, invalid("cursor", "Invalid cursor")
	}
	return cursor, nil
}

func jsonID(id int64) string {
	data, _ := json.Marshal(id)
	return string(data)
}

func snapshotPage[T any](ctx context.Context, s *Service, userID int64, operation string, limit int, raw string,
	load func() ([]T, error), validate func([]T) error) (Page[T], error) {
	if limit < 1 || limit > 100 {
		return Page[T]{}, invalid("limit", "Limit must be between 1 and 100")
	}
	var items []T
	cursor := pageCursor{Snapshot: randomToken("bip_"), Limit: limit}
	if raw == "" {
		var err error
		items, err = load()
		if err != nil {
			return Page[T]{}, err
		}
		encoded, err := json.Marshal(items)
		if err != nil {
			return Page[T]{}, err
		}
		_, err = s.db.ExecContext(ctx, `INSERT INTO bi_list_snapshots(id,user_id,operation,items,expires_at) VALUES($1,$2,$3,$4,$5)`,
			cursor.Snapshot, userID, operation, string(encoded), s.now().UTC().Add(30*time.Minute))
		if err != nil {
			return Page[T]{}, err
		}
	} else {
		var err error
		cursor, err = s.decodeCursor(raw, userID, operation, limit)
		if err != nil {
			return Page[T]{}, err
		}
		var data []byte
		err = s.db.QueryRowContext(ctx, `SELECT items FROM bi_list_snapshots WHERE id=$1 AND user_id=$2 AND operation=$3 AND expires_at>$4`,
			cursor.Snapshot, userID, operation, s.now().UTC()).Scan(&data)
		if errors.Is(err, sql.ErrNoRows) {
			return Page[T]{}, apiError(410, "CONTEXT_EXPIRED", "Pagination snapshot has expired")
		}
		if err != nil {
			return Page[T]{}, err
		}
		if err := json.Unmarshal(data, &items); err != nil {
			return Page[T]{}, err
		}
	}
	if err := validate(items); err != nil {
		return Page[T]{}, err
	}
	if cursor.Offset > len(items) {
		return Page[T]{}, invalid("cursor", "Cursor offset is out of range")
	}
	end := min(cursor.Offset+limit, len(items))
	page := Page[T]{Items: items[cursor.Offset:end], SnapshotID: cursor.Snapshot, HasMore: end < len(items)}
	if page.HasMore {
		cursor.Offset = end
		next := s.encodeCursor(cursor, userID, operation)
		page.NextCursor = &next
	}
	return page, nil
}

func (s *Service) ListOrganizations(ctx context.Context, p Principal, limit int, cursor string) (Page[Organization], error) {
	load := func() ([]Organization, error) { return s.organizations(ctx, s.db, p.ManagerID) }
	return snapshotPage(ctx, s, p.UserID, "listOrganizations", limit, cursor, load, func(snapshot []Organization) error {
		current, err := load()
		if err != nil {
			return err
		}
		byID := make(map[string]Organization, len(current))
		for _, org := range current {
			byID[org.ID] = org
		}
		for _, old := range snapshot {
			now, exists := byID[old.ID]
			if !exists || !containsAll(now.Capabilities, old.Capabilities) || (!now.AllTeams && (old.AllTeams || !containsAll(now.TeamIDs, old.TeamIDs))) {
				return apiError(403, "CONTEXT_REVOKED", "Pagination scope was revoked")
			}
		}
		return nil
	})
}

func containsAll(available, required []string) bool {
	for _, item := range required {
		if !slices.Contains(available, item) {
			return false
		}
	}
	return true
}

func (s *Service) bindings(ctx context.Context, userID int64) ([]BindingSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT b.id,b.created_at,b.last_login_at FROM bi_wechat_bindings b JOIN bi_managers m ON m.id=b.manager_id
		WHERE m.user_id=$1 AND b.appid=$2 AND b.revoked_at IS NULL ORDER BY b.id`, userID, s.config.AppID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []BindingSummary{}
	for rows.Next() {
		var item BindingSummary
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.LastLoginAt); err != nil {
			return nil, err
		}
		item.DisplayName = "得酷小程序"
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) ListBindings(ctx context.Context, userID int64, limit int, cursor string) (Page[BindingSummary], error) {
	load := func() ([]BindingSummary, error) { return s.bindings(ctx, userID) }
	return snapshotPage(ctx, s, userID, "listAccountBindings", limit, cursor, load, func(snapshot []BindingSummary) error {
		current, err := load()
		if err != nil {
			return err
		}
		allowed := make(map[string]bool, len(current))
		for _, b := range current {
			allowed[b.ID] = true
		}
		for _, b := range snapshot {
			if !allowed[b.ID] {
				return apiError(403, "CONTEXT_REVOKED", "Binding list changed; reload the first page")
			}
		}
		return nil
	})
}
