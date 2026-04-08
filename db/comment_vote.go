package db

// 投票：+1 有用 / -1 没用，切换逻辑
func VoteComment(commentID, userID int64, vote int) (upvotes, downvotes int, err error) {
	// 查当前投票
	var existing int
	err = GetDB().QueryRow(`SELECT vote FROM comment_votes WHERE comment_id = ? AND user_id = ?`, commentID, userID).Scan(&existing)
	if err == nil {
		if existing == vote {
			// 取消投票
			_, _ = GetDB().Exec(`DELETE FROM comment_votes WHERE comment_id = ? AND user_id = ?`, commentID, userID)
		} else {
			// 改投
			_, _ = GetDB().Exec(`UPDATE comment_votes SET vote = ? WHERE comment_id = ? AND user_id = ?`, vote, commentID, userID)
		}
	} else {
		// 新投票
		_, _ = GetDB().Exec(`INSERT INTO comment_votes (comment_id, user_id, vote) VALUES (?, ?, ?)`, commentID, userID, vote)
	}

	// 返回最新计数
	_ = GetDB().QueryRow(`SELECT COUNT(*) FROM comment_votes WHERE comment_id = ? AND vote = 1`, commentID).Scan(&upvotes)
	_ = GetDB().QueryRow(`SELECT COUNT(*) FROM comment_votes WHERE comment_id = ? AND vote = -1`, commentID).Scan(&downvotes)
	return
}

// 批量拿评论的投票数（搭配评论列表用）
func GetCommentVoteCounts(commentIDs []int64) map[int64][2]int {
	result := make(map[int64][2]int)
	if len(commentIDs) == 0 {
		return result
	}

	placeholders := make([]string, len(commentIDs))
	args := make([]interface{}, len(commentIDs))
	for i, id := range commentIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := GetDB().Query(`
		SELECT comment_id, vote, COUNT(*) FROM comment_votes
		WHERE comment_id IN (`+joinStrings(placeholders, ",")+`)
		GROUP BY comment_id, vote
	`, args...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var cid int64
		var vote, count int
		if rows.Scan(&cid, &vote, &count) == nil {
			v := result[cid]
			if vote == 1 {
				v[0] = count
			} else {
				v[1] = count
			}
			result[cid] = v
		}
	}
	return result
}
