package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"skills-hub/db"
)

// POST /admin/skills/batch-review — 批量审核
func AdminBatchReviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	status := strings.TrimSpace(r.FormValue("review_status"))
	if status != "approved" && status != "rejected" {
		http.Redirect(w, r, "/admin/skills?error=审核状态不正确", http.StatusSeeOther)
		return
	}

	reviewNote := strings.TrimSpace(r.FormValue("review_note"))
	adminID := adminActorID(r)

	idsStr := strings.TrimSpace(r.FormValue("skill_ids"))
	if idsStr == "" {
		http.Redirect(w, r, "/admin/skills?error=请选择要审核的技能", http.StatusSeeOther)
		return
	}

	count := 0
	for _, idStr := range strings.Split(idsStr, ",") {
		id, err := parseInt64(strings.TrimSpace(idStr))
		if err != nil || id <= 0 {
			continue
		}
		if err := db.ReviewAdminSkill(id, adminID, status, reviewNote); err != nil {
			slog.Error("batch review failed", "skill_id", id, "error", err)
			continue
		}
		count++
		go notifySkillReviewResult(id, status, reviewNote)
	}

	action := "skill.batch_approve"
	if status == "rejected" {
		action = "skill.batch_reject"
	}
	recordAdminAction(r, action, "skill", 0, fmt.Sprintf("%s (%s)", reviewNote, idsStr))

	http.Redirect(w, r, fmt.Sprintf("/admin/skills?info=已批量审核 %d 个技能", count), http.StatusSeeOther)
}
