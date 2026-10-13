package bi

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func daysQuery(c *gin.Context) (int, bool) {
	days, e := strconv.Atoi(c.DefaultQuery("days", "30"))
	if e != nil || (days != 7 && days != 30 && days != 90) {
		WriteError(c, invalid("days", "Expected 7, 30 or 90"))
		return 0, false
	}
	return days, true
}
func conversationView(v service.DesktopConversationMetadata) gin.H {
	return gin.H{"id": v.RecordID, "member_id": v.MemberID, "member_name": v.MemberName, "member_deleted": v.MemberDeleted, "client": v.Client, "started_at": v.StartedAt, "stopped_at": v.StoppedAt, "received_at": v.ReceivedAt, "duration_ms": v.DurationMS, "capture_status": v.CaptureStatus, "prompt_count": v.PromptCount}
}
func countsView(v *service.DesktopConversationStatistics) gin.H {
	return gin.H{"record_count": v.Total.RecordCount, "prompt_count": v.Total.PromptCount, "distinct_members": v.DistinctMembers, "distinct_sessions": v.DistinctSessions, "captured": v.Captured, "response_missing": v.ResponseMissing}
}
func statisticsView(v *service.DesktopConversationStatistics) gin.H {
	return gin.H{"range": gin.H{"start_date": v.RangeStart, "end_date": v.RangeEnd, "timezone": v.Timezone, "includes_today": true}, "as_of": v.AsOf, "counts": countsView(v), "daily": v.Daily}
}
func dailyMetadata(v *service.DesktopReportDay) gin.H {
	var revision any
	id := "day:" + v.Date
	if v.Task != nil {
		revision = v.Task.Revision
	}
	return gin.H{"id": id, "kind": "day", "range": gin.H{"start_date": v.Date, "end_date": v.Date, "timezone": v.Timezone, "includes_today": false}, "revision": revision, "status": v.Status, "reason": v.Reason, "generated_at": v.Summary.GeneratedAt, "expected_members": v.ExpectedMembers, "completed_members": v.CompletedMembers, "coverage": gin.H{"missing_members": len(v.MissingMembers)}, "source_changed": false}
}
func (h *Handler) NativeRead(operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		p := principal(c)
		org := c.Param("organization_id")
		required := []string{"analytics:read"}
		switch operation {
		case "organization":
			required = nil
		case "members":
			required = []string{"members:read"}
		case "member":
			required = []string{"members:read"}
		case "member_models":
			required = []string{"members:read", "analytics:read"}
		case "conversations", "conversation":
			required = []string{"members:read", "conversations:read"}
		case "daily", "daily_summary", "daily_members", "daily_member", "periods", "period", "sources", "source":
			required = []string{"members:read", "reports:read"}
		case "daily_text", "period_text":
			required = []string{"members:read", "reports:read", "reports:copy"}
		}
		access, err := h.service.AuthorizeOrganization(ctx, p, org, required...)
		if err != nil {
			WriteError(c, err)
			return
		}
		if h.service.desktop == nil || !h.desktopEnabled {
			WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Desktop unavailable"))
			return
		}
		organization, err := h.service.desktop.GetOrganization(ctx, org)
		if err != nil {
			WriteError(c, err)
			return
		}
		has := func(cap string) bool { return slices.Contains(access.Capabilities, cap) }
		if operation == "organization" {
			features := gin.H{}
			for feature, cap := range map[string]string{"analytics": "analytics:read", "members": "members:read", "conversations": "conversations:read", "reports": "reports:read", "copy": "reports:copy", "uploads": "analytics:read"} {
				state := "available"
				reason := ""
				if !has(cap) {
					state = "forbidden"
					reason = "permission_required"
				}
				if ((feature == "conversations" || feature == "uploads") && !organization.ConversationReportingEnabled) || (feature == "reports" && !organization.ConversationSummaryEnabled) {
					state = "disabled"
					reason = "configuration_disabled"
				}
				features[feature] = gin.H{"status": state, "reason": reason}
			}
			for _, feature := range []string{"adoption", "retention", "departments", "scenes", "knowledge", "evaluations", "sharing"} {
				features[feature] = gin.H{"status": "unsupported", "reason": "not_connected"}
			}
			current, e := h.service.AuthorizeOrganization(ctx, p, org)
			if e != nil || current.Revision != access.Revision {
				WriteError(c, ErrForbidden)
				return
			}
			c.JSON(200, gin.H{"id": org, "name": organization.Name, "status": organization.Status, "scope": "organization", "capabilities": access.Capabilities, "revision": access.Revision, "features": features})
			return
		}
		if (strings.HasPrefix(operation, "daily") || strings.HasPrefix(operation, "period") || operation == "sources" || operation == "source") && !organization.ConversationSummaryEnabled {
			WriteError(c, apiError(403, "FEATURE_DISABLED", "Enterprise summaries are disabled"))
			return
		}
		if (operation == "clients" || operation == "client" || strings.HasPrefix(operation, "conversation")) && !organization.ConversationReportingEnabled {
			WriteError(c, apiError(403, "FEATURE_DISABLED", "Conversation reporting is disabled"))
			return
		}
		days, ok := daysQuery(c)
		if !ok {
			return
		}
		page, size, ok := queryPage(c)
		if !ok {
			return
		}
		var result any
		switch operation {
		case "usage", "models", "members", "member", "member_models":
			reader, o, selection, e := h.service.desktop.NativeAnalytics(ctx, org, days)
			if e != nil {
				err = e
				break
			}
			switch operation {
			case "usage":
				result, err = reader.NativeUsage(ctx, o.ID, selection)
			case "models":
				var items []service.DesktopNativeModel
				var total int64
				items, total, err = reader.NativeModels(ctx, o.ID, selection, "", page, size)
				result = pageResult(items, page, size, total)
			case "members":
				if c.Query("status") != "" && c.Query("status") != "active" && c.Query("status") != "disabled" {
					err = invalid("status", "Invalid member status")
					break
				}
				if len([]rune(c.Query("search"))) > 100 {
					err = invalid("search", "Search exceeds 100 characters")
					break
				}
				var items []service.DesktopNativeMember
				var total int64
				items, total, err = reader.NativeMembers(ctx, o.ID, selection, c.Query("search"), c.Query("status"), page, size, has("analytics:read"))
				result = pageResult(items, page, size, total)
			case "member", "member_models":
				var member *service.DesktopNativeMember
				member, err = reader.NativeMember(ctx, o.ID, selection, c.Param("member_id"), has("analytics:read"))
				if err != nil {
					break
				}
				if operation == "member_models" {
					models, total, e := reader.NativeModels(ctx, o.ID, selection, c.Param("member_id"), page, size)
					err = e
					result = pageResult(models, page, size, total)
				} else {
					result = gin.H{"member": member, "range": service.DesktopNativeRange{Start: selection.StartDate, End: selection.EndDate, Timezone: selection.Start.Location().String(), IncludesToday: true}}
				}

			}
		case "overview":
			var usage *service.DesktopNativeUsage
			usage, err = h.service.desktop.NativeUsage(ctx, org, days)
			if err != nil {
				break
			}
			var conversations any
			conversationStatus := "disabled"
			summaryStatus := "forbidden"
			if organization.ConversationReportingEnabled {
				stats, e := h.service.desktop.ConversationStatistics(ctx, org, 0, service.DesktopConversationFilters{AnalyticsRange: &service.DesktopAnalyticsRangeInput{Days: days, Timezone: "Asia/Shanghai"}})
				conversationStatus = "error"
				if e == nil {
					conversations = statisticsView(stats)
					conversationStatus = "available"
				}
			}
			summaries := []any{}
			if has("reports:read") && has("members:read") && organization.ConversationSummaryEnabled {
				day, e := h.service.desktop.ReadLatestReportDay(ctx, org)
				summaryStatus = "error"
				if e == nil {
					summaryStatus = "available"
					summaries = append(summaries, dailyMetadata(day))
				}
			}
			result = gin.H{"range": usage.Range, "as_of": usage.AsOf, "observed_members": usage.ObservedMembers, "total_tokens": usage.Totals.Total, "usage": usage, "conversations": conversations, "latest_summaries": summaries, "module_status": gin.H{"conversations": conversationStatus, "summaries": summaryStatus}}
		case "conversation_statistics", "clients", "client":
			filter := service.DesktopConversationFilters{AnalyticsRange: &service.DesktopAnalyticsRangeInput{Days: days, Timezone: "Asia/Shanghai"}, MemberID: c.Query("member_id"), Client: c.Query("client"), CaptureStatus: c.Query("capture_status")}
			if operation == "client" {
				filter.Client = c.Param("client")
			}
			stats, e := h.service.desktop.ConversationStatistics(ctx, org, 0, filter)
			err = e
			if e != nil {
				break
			}
			result = statisticsView(stats)
			if operation == "clients" {
				result = gin.H{"range": statisticsView(stats)["range"], "items": []gin.H{{"client": "workbuddy", "record_count": stats.Workbuddy}, {"client": "chatgpt_codex", "record_count": stats.ChatGPTCodex}}}
			}
			if operation == "client" {
				result = gin.H{"client": filter.Client, "statistics": statisticsView(stats)}
			}
		case "conversations":
			input := service.DesktopAnalyticsRangeInput{Days: days, Timezone: "Asia/Shanghai"}
			selected, e := input.Resolve(h.service.now())
			if e != nil {
				err = e
				break
			}
			filters := service.DesktopConversationFilters{MemberID: c.Query("member_id"), MemberSearch: c.Query("search"), Client: c.Query("client"), CaptureStatus: c.Query("capture_status"), ReceivedFrom: &selected.Start, ReceivedTo: &selected.End}
			items, paginationResult, e := h.service.desktop.ListConversations(ctx, org, 0, pagination.PaginationParams{Page: page, PageSize: size, SortOrder: "desc"}, filters)
			err = e
			if e != nil {
				break
			}
			values := []gin.H{}
			for _, item := range items {
				values = append(values, conversationView(item))
			}
			result = pageResult(values, page, size, int64(paginationResult.Total))
		case "conversation":
			value, e := h.service.desktop.GetConversation(ctx, org, 0, c.Param("record_id"))
			err = e
			if e != nil {
				break
			}
			v := conversationView(value.DesktopConversationMetadata)
			v["prompts"] = value.Prompts
			v["response"] = value.Response
			result = v
		case "daily", "daily_summary", "daily_members", "daily_member", "daily_text":
			date := c.Param("date")
			if date == "" {
				date = c.Query("date")
			}
			section := "metadata"
			if operation == "daily_summary" || operation == "daily_text" {
				section = "summary"
			}
			if operation == "daily_members" || operation == "daily_member" {
				section = "members"
			}
			day, e := h.service.desktop.ReadReportDay(ctx, org, 0, date, service.DesktopReportQuery{Section: section, MemberID: c.Param("member_id"), Search: c.Query("search"), Page: page, PageSize: size, ExecutionPage: 1})
			if operation == "daily" && c.Query("latest") == "true" {
				day, e = h.service.desktop.ReadLatestReportDay(ctx, org)
			}
			err = e
			if e != nil {
				break
			}
			switch operation {
			case "daily":
				result = dailyMetadata(day)
			case "daily_summary":
				v := dailyMetadata(day)
				v["content"] = day.Summary.Content
				v["can_copy"] = has("reports:copy")
				result = v
			case "daily_text":
				if day.Summary.Content == "" {
					err = apiError(409, "CONFLICT", "Summary not ready")
					break
				}
				result = gin.H{"text": day.Summary.Content, "revision": dailyMetadata(day)["revision"]}
			case "daily_members", "daily_member":
				items := []gin.H{}
				for _, m := range day.Members {
					value := gin.H{"member_id": m.MemberID, "name": m.Name, "deleted": m.Deleted, "date": day.Date, "status": m.Status, "record_count": m.RecordCount, "generated_at": m.GeneratedAt}
					if operation == "daily_member" {
						value["content"] = m.Content
						value["source_ids"] = m.SourceIDs
					}
					items = append(items, value)
				}
				if operation == "daily_member" {
					if len(items) == 0 {
						err = ErrNotFound
					} else {
						result = items[0]
					}
				} else {
					result = pageResult(items, page, size, int64(day.TotalMembers))
				}
			}
		case "periods", "period", "sources", "source", "period_text":
			result, err = h.readPeriod(c, access, operation, page, size)
		default:
			err = ErrNotFound
		}
		if err != nil {
			WriteError(c, err)
			return
		}
		current, e := h.service.AuthorizeOrganization(ctx, p, org, required...)
		if e != nil || current.Revision != access.Revision {
			WriteError(c, ErrForbidden)
			return
		}
		respond(c, 200, result, nil)
	}
}

// jsonObject projects structured metadata without exposing unselected source fields.
func jsonObject(v any) gin.H {
	raw, _ := json.Marshal(v)
	out := gin.H{}
	_ = json.Unmarshal(raw, &out)
	return out
}
