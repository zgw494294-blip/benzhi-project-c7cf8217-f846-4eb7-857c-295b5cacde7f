package domain

import (
	"fmt"
	"sort"
	"strings"
)

func ValidateMetadata(title, shelfMark string) error {
	if strings.TrimSpace(title) == "" {
		return NewRuleError(CodeInvalid, "题名不能为空")
	}
	if strings.TrimSpace(shelfMark) == "" {
		return NewRuleError(CodeInvalid, "架藏号不能为空")
	}
	if len([]rune(title)) > 200 || len([]rune(shelfMark)) > 100 {
		return NewRuleError(CodeInvalid, "书目信息超过允许长度")
	}
	return nil
}

func ValidatePageOrder(v *DigitizationVolume, order []string) error {
	if len(order) != len(v.Pages) {
		return NewRuleError(CodeInvalid, "页序必须包含卷内全部页面")
	}
	known := make(map[string]bool, len(v.Pages))
	for _, page := range v.Pages {
		known[page.ID] = true
	}
	seen := make(map[string]bool, len(order))
	for _, id := range order {
		if !known[id] {
			return NewRuleError(CodeInvalid, "页序包含未知页面 %s", id)
		}
		if seen[id] {
			return NewRuleError(CodeInvalid, "页序重复包含页面 %s", id)
		}
		seen[id] = true
	}
	return nil
}

func ValidateFinding(category FindingCategory, location string) error {
	valid := map[FindingCategory]bool{
		CategoryMissingGlyph: true, CategoryVariant: true,
		CategoryLayoutBreak: true, CategoryFolio: true,
	}
	if !valid[category] {
		return NewRuleError(CodeInvalid, "不支持的疑难类别 %s", category)
	}
	if strings.TrimSpace(location) == "" {
		return NewRuleError(CodeInvalid, "疑难位置不能为空")
	}
	return nil
}

func CheckIntegrity(v *DigitizationVolume) []IntegrityViolation {
	violations := make([]IntegrityViolation, 0)
	pages := OrderedPages(v)
	if len(pages) == 0 {
		violations = append(violations, IntegrityViolation{Code: "NO_PAGES", Severity: SeverityBlocker, Message: "卷内尚无扫描页"})
	}
	folios := make(map[string]string)
	for index, page := range pages {
		label := strings.TrimSpace(page.FolioLabel)
		if label == "" {
			violations = append(violations, IntegrityViolation{Code: "MISSING_FOLIO", Severity: SeverityBlocker, PageID: page.ID, Message: "页面缺少叶号"})
		} else if firstID, exists := folios[label]; exists {
			violations = append(violations, IntegrityViolation{Code: "DUPLICATE_FOLIO", Severity: SeverityBlocker, PageID: page.ID, FolioLabel: label, Message: fmt.Sprintf("叶号 %s 与页面 %s 重复", label, firstID)})
		} else {
			folios[label] = page.ID
		}
		if page.Sequence != index+1 {
			violations = append(violations, IntegrityViolation{Code: "SEQUENCE_GAP", Severity: SeverityBlocker, PageID: page.ID, FolioLabel: label, Message: fmt.Sprintf("页序应为 %d，实际为 %d", index+1, page.Sequence)})
		}
		if strings.TrimSpace(page.Transcription) == "" {
			violations = append(violations, IntegrityViolation{Code: "EMPTY_TRANSCRIPTION", Severity: SeverityBlocker, PageID: page.ID, FolioLabel: label, Message: "页面尚未录入转录"})
		}
		if len(page.SHA256) != 64 || page.ImageObjectKey == "" {
			violations = append(violations, IntegrityViolation{Code: "MISSING_IMAGE_DIGEST", Severity: SeverityBlocker, PageID: page.ID, FolioLabel: label, Message: "扫描图像摘要缺失或无效"})
		}
		if page.Width <= 0 || page.Height <= 0 {
			violations = append(violations, IntegrityViolation{Code: "IMAGE_DIMENSIONS_UNKNOWN", Severity: SeverityWarning, PageID: page.ID, FolioLabel: label, Message: "图像尺寸未能识别"})
		}
	}
	for _, finding := range v.Findings {
		if finding.Status == FindingOpen {
			violations = append(violations, IntegrityViolation{Code: "OPEN_FINDING", Severity: SeverityBlocker, PageID: finding.PageID, Message: "仍有未决疑难字或版面问题：" + finding.Location})
		}
	}
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].PageID == violations[j].PageID {
			return violations[i].Code < violations[j].Code
		}
		return violations[i].PageID < violations[j].PageID
	})
	return violations
}

func HasBlockers(violations []IntegrityViolation) bool {
	for _, violation := range violations {
		if violation.Severity == SeverityBlocker {
			return true
		}
	}
	return false
}
