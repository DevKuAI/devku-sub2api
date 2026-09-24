package bi

import (
	"context"
)

// CleanupEphemeralState removes expired credentials and read caches in bounded batches.
// Business facts, content versions and audit history are not temporary caches.
func (s *Service) CleanupEphemeralState(ctx context.Context) (int64, error) {
	now := s.now().UTC()
	ephermeralDays := defaultEphemeralRetentionDays
	if policy, err := s.retentionPolicy(ctx); err == nil && policy.EphemeralDays > 0 {
		ephermeralDays = policy.EphemeralDays
	}
	cutoff := now.AddDate(0, 0, -ephermeralDays)
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM bi_refresh_tokens WHERE token_hash IN (SELECT token_hash FROM bi_refresh_tokens WHERE expires_at<=$1 LIMIT 5000)`, []any{now}},
		{`DELETE FROM bi_sessions WHERE id IN (SELECT s.id FROM bi_sessions s WHERE s.expires_at<=$1 AND NOT EXISTS(SELECT 1 FROM bi_refresh_tokens r WHERE r.session_id=s.id) LIMIT 5000)`, []any{now}},
		{`DELETE FROM bi_binding_challenges WHERE ticket_hash IN (SELECT ticket_hash FROM bi_binding_challenges WHERE expires_at<=$1 LIMIT 5000)`, []any{cutoff}},
		{`DELETE FROM bi_wechat_codes WHERE code_hash IN (SELECT code_hash FROM bi_wechat_codes WHERE created_at<=$1 LIMIT 5000)`, []any{cutoff}},
		{`DELETE FROM bi_list_snapshots WHERE id IN (SELECT id FROM bi_list_snapshots WHERE expires_at<=$1 LIMIT 5000)`, []any{now}},
		{`DELETE FROM bi_analysis_contexts WHERE id IN (SELECT id FROM bi_analysis_contexts WHERE expires_at<=$1 LIMIT 5000)`, []any{now}},
		{`DELETE FROM bi_command_receipts WHERE ctid IN (SELECT ctid FROM bi_command_receipts WHERE expires_at<=$1 LIMIT 5000)`, []any{now}},
	}
	var total int64
	for _, statement := range statements {
		result, err := s.db.ExecContext(ctx, statement.query, statement.args...)
		if err != nil {
			return total, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return total, err
		}
		total += count
	}
	return total, nil
}
