package review

import "fmt"

// truncateDiff 截断过长的 diff
func truncateDiff(diff string, maxLen int) string {
	if len(diff) <= maxLen {
		return diff
	}
	return diff[:maxLen] + fmt.Sprintf("\n\n... (diff 已截断，原始长度 %d 字符)", len(diff))
}
