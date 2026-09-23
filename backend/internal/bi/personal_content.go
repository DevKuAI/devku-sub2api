package bi

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) liveKnowledge(ctx context.Context, p Principal, org, id string) error {
	scope, err := s.AuthorizeOrganization(ctx, p, org, "knowledge:read")
	if err != nil {
		return err
	}
	state := analysisState{Principal: p, Scope: scope, Revision: -1, Context: AnalysisContext{OrganizationID: org, Filters: Filters{TeamIDs: []string{}}}}
	_, err = visibleRecord(ctx, s.db, state, "knowledge", id)
	return err
}

func (s *Service) SetFavorite(ctx context.Context, p Principal, org, id string) error {
	if err := s.liveKnowledge(ctx, p, org, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO bi_favorites(manager_id,organization_id,knowledge_id,created_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, p.ManagerID, org, id, s.now().UTC())
	return err
}

func (s *Service) RemoveFavorite(ctx context.Context, p Principal, org, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bi_favorites WHERE manager_id=$1 AND organization_id=$2 AND knowledge_id=$3`, p.ManagerID, org, id)
	return err
}

func readNote(ctx context.Context, q queryer, p Principal, org, id string) (*KnowledgeNote, error) {
	var note KnowledgeNote
	var updated time.Time
	err := q.QueryRowContext(ctx, `SELECT knowledge_id,text,revision,updated_at FROM bi_knowledge_notes WHERE manager_id=$1 AND organization_id=$2 AND knowledge_id=$3`, p.ManagerID, org, id).
		Scan(&note.KnowledgeID, &note.Text, &note.Revision, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	note.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return &note, nil
}

func (s *Service) ReadNote(ctx context.Context, p Principal, org, id string) (*KnowledgeNote, error) {
	if err := s.liveKnowledge(ctx, p, org, id); err != nil {
		return nil, err
	}
	return readNote(ctx, s.db, p, org, id)
}

type noteCondition struct {
	create, any bool
	tags        []string
}

func parseNoteCondition(match, none string, deleting bool) (noteCondition, error) {
	var result noteCondition
	match, none = strings.TrimSpace(match), strings.TrimSpace(none)
	if match == "" && none == "" {
		return result, apiError(428, "PRECONDITION_REQUIRED", "A conditional request header is required")
	}
	if match != "" && none != "" {
		return result, invalid("headers", "If-Match and If-None-Match are mutually exclusive")
	}
	if none != "" {
		if deleting || none != "*" {
			return result, invalid("If-None-Match", "Only creation with * is supported")
		}
		result.create = true
		return result, nil
	}
	if match == "*" {
		result.any = true
		return result, nil
	}
	for len(match) > 0 {
		match = strings.TrimSpace(match)
		weak := strings.HasPrefix(match, "W/")
		if weak {
			match = match[2:]
		}
		if len(match) < 2 || match[0] != '"' {
			return result, invalid("If-Match", "Expected an entity tag")
		}
		end := strings.IndexByte(match[1:], '"')
		if end < 0 {
			return result, invalid("If-Match", "Unterminated entity tag")
		}
		end++
		tag := match[1:end]
		for _, ch := range []byte(tag) {
			if ch < 0x21 || ch == 0x7f {
				return result, invalid("If-Match", "Invalid entity tag")
			}
		}
		if !weak {
			result.tags = append(result.tags, tag)
		}
		match = strings.TrimSpace(match[end+1:])
		if match == "" {
			break
		}
		if match[0] != ',' {
			return result, invalid("If-Match", "Invalid entity tag list")
		}
		match = match[1:]
		if strings.TrimSpace(match) == "" {
			return result, invalid("If-Match", "Missing entity tag")
		}
	}
	return result, nil
}

func (condition noteCondition) matches(note *KnowledgeNote) bool {
	if condition.create {
		return note == nil
	}
	if note == nil {
		return false
	}
	if condition.any {
		return true
	}
	for _, tag := range condition.tags {
		if tag == note.Revision {
			return true
		}
	}
	return false
}

func (s *Service) mutateNote(ctx context.Context, p Principal, org, id, text, match, none string, deleting bool) (*KnowledgeNote, error) {
	condition, err := parseNoteCondition(match, none, deleting)
	if err != nil {
		return nil, err
	}
	if !deleting {
		if !utf8.ValidString(text) || utf8.RuneCountInString(text) < 1 || utf8.RuneCountInString(text) > 300 {
			return nil, invalid("text", "Note must contain 1–300 characters")
		}
		if err := s.liveKnowledge(ctx, p, org, id); err != nil {
			return nil, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "bi:note:"+p.ManagerID+":"+org+":"+id); err != nil {
		return nil, err
	}
	current, err := readNote(ctx, tx, p, org, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if deleting && current == nil {
		return nil, nil
	}
	if !condition.matches(current) {
		return nil, apiError(412, "PRECONDITION_FAILED", "Note changed; reload before saving")
	}
	if deleting {
		if _, err := tx.ExecContext(ctx, `DELETE FROM bi_knowledge_notes WHERE manager_id=$1 AND organization_id=$2 AND knowledge_id=$3`, p.ManagerID, org, id); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	revision := randomToken("note_")
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_knowledge_notes(manager_id,organization_id,knowledge_id,text,revision,updated_at) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(manager_id,organization_id,knowledge_id) DO UPDATE SET text=EXCLUDED.text,revision=EXCLUDED.revision,updated_at=EXCLUDED.updated_at`, p.ManagerID, org, id, text, revision, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &KnowledgeNote{KnowledgeID: id, Text: text, Revision: revision, UpdatedAt: now.Format(time.RFC3339Nano)}, nil
}
