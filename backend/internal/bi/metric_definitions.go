package bi

func metricDefinitions() []MetricDefinition {
	rows := []struct{ key, name, unit, definition string }{
		{"eligible_members", "可使用人员", "person", "期末在授权范围内、符合有效 Eligibility 的人员去重数；不能从实际调用者反推。"},
		{"activated_members", "已激活人员", "person", "期末可使用人员中，截至期末有成功人员交互的人数；历史不完整时未知。"},
		{"active_members", "活跃人员", "person", "期末可使用人员中，本期至少一次成功 human 交互的不同人员数。"},
		{"stable_members", "稳定使用人员", "person", "期末可使用人员中，最近四个完整周至少三周有成功人员交互的人数。"},
		{"repeat_members", "重复使用人员", "person", "期末可使用人员中，本期至少两个不同上海自然日有成功人员交互的人数。"},
		{"adoption_rate", "采用率", "ratio", "本期活跃人员数除以期末可使用人员数；分母为零时未知。"},
		{"frequency", "使用频率", "person", "按成功人员交互的不同自然日数，分为 0、1、2–3、4 天及以上四个互斥桶。"},
		{"weekly_retention", "首次使用周留存", "ratio", "固定首次成功交互队列，在后续第 1、2、3 个完整周继续成功交互的人员比例；调岗与离职不重建分母。"},
		{"total_tokens", "Token 总量", "token", "标准 input 与 output 之和；cache_read 和 cache_write 是 input 子集，不再重复累加。"},
		{"human_tokens", "人员交互 Token", "token", "显式 actor_type=human 的已测量调用用量，包含已测量失败尝试。"},
		{"automatic_tokens", "自动执行 Token", "token", "显式 actor_type=automatic 的已测量调用用量，不代表人员采用。"},
		{"unknown_tokens", "未知归因 Token", "token", "显式 actor_type=unknown 的已测量调用用量，不推断为人员或自动执行。"},
		{"non_cohort_human_tokens", "非期末母体人员 Token", "token", "本期人员交互中，不属于期末可使用人员母体的已测量用量。"},
		{"request_count", "调用尝试数", "request", "实际调用尝试的已观测数量，重试分别计数；未知结果另列。"},
		{"failed_requests", "失败尝试数", "request", "已知终态为 failed 的调用尝试数。"},
		{"failure_rate", "失败率", "ratio", "failed 除以 succeeded 与 failed 之和；unknown 结果不进入分母。"},
		{"rated_interactions", "已评价人员交互", "request", "本期人员交互中具有当前已采纳评价的交互数；每次交互一票，修订替换。"},
		{"helpful_rate", "有帮助比例", "ratio", "helpful 评价数除以已评价交互数；无评价时未知。"},
		{"feedback_coverage", "评价覆盖率", "ratio", "已评价人员交互数除以全部人员交互数；个人知识 note 不进入评价。"},
		{"valid_knowledge", "有效知识", "knowledge", "内容快照时点可见且状态为 valid 的知识数，应用与场景筛选按适用范围取交集。"},
		{"reused_valid_knowledge", "复用的有效知识", "knowledge", "当前可见有效知识中，本期被成功调用实际引用的不同知识数。"},
		{"knowledge_reuse_rate", "知识复用率", "ratio", "复用的有效知识数除以当前可见有效知识数；分母为零时未知。"},
		{"knowledge_references", "知识引用次数", "request", "成功调用中按 event_id 去重的知识引用次数；同一知识多个版本不重复计数。"},
		{"stale_referenced_now", "当前过期且本期被引用的知识", "knowledge", "当前已过期知识中，本期有成功引用的不同知识数；不等同于引用当时已过期。"},
		{"expired_at_use", "引用时已过期的知识", "knowledge", "根据知识状态历史，成功调用发生时已过期且有实际引用的不同知识数。"},
		{"added_valid", "新增有效知识", "knowledge", "当前有效且本期创建的知识数。"},
		{"updated_valid", "更新有效知识", "knowledge", "当前有效、在本期之前创建且本期有实质版本更新的知识数，与新增互斥。"},
		{"evaluation_pass_rate", "评估通过率", "ratio", "同一完整测试集与标准下，passed 样本数除以全部样本数；not_run 不计通过。"},
	}
	result := make([]MetricDefinition, 0, len(rows))
	for _, row := range rows {
		result = append(result, MetricDefinition{Key: row.key, Name: row.name, Definition: row.definition, Unit: row.unit, Version: "bi-v1", Limitations: []string{"未知值与真实零值分开；不完整数据不得生成完整覆盖或增长结论。"}})
	}
	return result
}
