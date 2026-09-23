package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// dayRevisionSQL chooses one complete day view per date within a frozen revision.
const dayRevisionSQL = `SELECT DISTINCT ON (day.day) day.day,day.data_revision FROM bi_usage_day_revisions day
 JOIN bi_data_revisions revision ON revision.id=day.data_revision
 WHERE day.organization_id=$1 AND day.data_revision<=$2 AND revision.status='published'
 AND day.day>=$3::date AND day.day<$4::date ORDER BY day.day,day.data_revision DESC`

type dailyUsage struct {
	SourceID                                                   string
	TeamID                                                     *string
	Actor                                                      string
	ApplicationID, SceneID                                     *string
	Model                                                      string
	Requests, Succeeded, Failed, Unknown, Measured, Unmeasured int64
	Tokens                                                     NormalizedTokens
}

func buildDailyUsage(ctx context.Context, tx *sql.Tx, org string, revision int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT (f.occurred_at AT TIME ZONE 'Asia/Shanghai')::date
		FROM bi_usage_facts f JOIN bi_data_revisions d ON d.id=f.data_revision
		WHERE f.organization_id=$1 AND (d.status='published' OR f.data_revision=$2)
		AND f.entity_id IN (SELECT entity_id FROM bi_usage_facts WHERE organization_id=$1 AND data_revision=$2)`, org, revision)
	if err != nil {
		return err
	}
	days := []time.Time{}
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			_ = rows.Close()
			return err
		}
		days = append(days, day)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, day := range days {
		if err := buildUsageDay(ctx, tx, org, revision, day.Format("2006-01-02")); err != nil {
			return err
		}
	}
	return nil
}

func buildUsageDay(ctx context.Context, tx *sql.Tx, org string, revision int64, day string) error {
	start, err := time.ParseInLocation("2006-01-02", day, shanghai)
	if err != nil {
		return err
	}
	end := start.AddDate(0, 0, 1)
	rows, err := tx.QueryContext(ctx, `SELECT e.source_id,f.team_id,f.actor_type,f.application_id,f.scene_id,f.requested_model,
		COUNT(*),COUNT(*) FILTER(WHERE f.outcome='succeeded'),COUNT(*) FILTER(WHERE f.outcome='failed'),COUNT(*) FILTER(WHERE f.outcome='unknown'),
		COUNT(f.input_tokens),COUNT(*) FILTER(WHERE f.input_tokens IS NULL),SUM(f.input_tokens)::text,SUM(f.output_tokens)::text,SUM(f.cache_read_tokens)::text,SUM(f.cache_write_tokens)::text
		FROM (SELECT DISTINCT ON (u.entity_id) u.* FROM bi_usage_facts u JOIN bi_data_revisions d ON d.id=u.data_revision
		WHERE u.organization_id=$1 AND (d.status='published' OR u.data_revision=$2)
		AND u.entity_id IN (SELECT entity_id FROM bi_usage_facts WHERE organization_id=$1 AND occurred_at>=$3 AND occurred_at<$4)
		ORDER BY u.entity_id,u.data_revision DESC,u.entity_revision DESC) f
		JOIN bi_entities e ON e.organization_id=f.organization_id AND e.kind='usage' AND e.id=f.entity_id
		WHERE NOT f.tombstone AND f.occurred_at>=$3 AND f.occurred_at<$4
		GROUP BY e.source_id,f.team_id,f.actor_type,f.application_id,f.scene_id,f.requested_model`, org, revision, start, end)
	if err != nil {
		return err
	}
	groups := []dailyUsage{}
	for rows.Next() {
		var group dailyUsage
		if err := rows.Scan(&group.SourceID, &group.TeamID, &group.Actor, &group.ApplicationID, &group.SceneID, &group.Model, &group.Requests, &group.Succeeded, &group.Failed, &group.Unknown, &group.Measured, &group.Unmeasured,
			&group.Tokens.Input, &group.Tokens.Output, &group.Tokens.CacheRead, &group.Tokens.CacheWrite); err != nil {
			_ = rows.Close()
			return err
		}
		groups = append(groups, group)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	// An empty marker must be published too, so moving/retracting the last event
	// replaces the previous day's groups instead of making them reappear.
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_usage_day_revisions(organization_id,day,data_revision) VALUES($1,$2,$3)`, org, day, revision); err != nil {
		return err
	}
	for _, group := range groups {
		dimensions, _ := json.Marshal([]any{group.SourceID, group.TeamID, group.Actor, group.ApplicationID, group.SceneID, group.Model})
		_, err := tx.ExecContext(ctx, `INSERT INTO bi_usage_daily(organization_id,day,data_revision,dimension_hash,source_id,team_id,actor_type,application_id,scene_id,requested_model,
			requests,succeeded,failed,unknown_outcome,measured,unmeasured,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, org, day, revision, tokenHash(string(dimensions)), group.SourceID, group.TeamID, group.Actor, group.ApplicationID, group.SceneID, group.Model,
			group.Requests, group.Succeeded, group.Failed, group.Unknown, group.Measured, group.Unmeasured, group.Tokens.Input, group.Tokens.Output, group.Tokens.CacheRead, group.Tokens.CacheWrite)
		if err != nil {
			return err
		}
	}
	return nil
}
