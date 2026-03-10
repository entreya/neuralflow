package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// EvalResult holds the outcome of evaluating an LLM output.
type EvalResult struct {
	Score      float64  `json:"score"`
	Passed     bool     `json:"passed"`
	Errors     []string `json:"errors"`
	Tier1Score float64  `json:"tier1_score"`
	Tier2Score float64  `json:"tier2_score"`
}

// Evaluate runs both universal (tier 1) and inferred (tier 2) rules against
// the raw LLM output string and returns a composite score.
func Evaluate(rawOutput string, rules []Rule) EvalResult {
	tier1Score, tier1Errors := evaluateTier1(rawOutput)
	tier2Score, tier2Errors := evaluateTier2(rawOutput, rules)

	allErrors := make([]string, 0, len(tier1Errors)+len(tier2Errors))
	allErrors = append(allErrors, tier1Errors...)
	allErrors = append(allErrors, tier2Errors...)

	// Final score = (tier1 * 0.4) + (tier2 * 0.6)
	finalScore := (tier1Score * 0.4) + (tier2Score * 0.6)

	return EvalResult{
		Score:      finalScore,
		Passed:     finalScore >= 0.75,
		Errors:     allErrors,
		Tier1Score: tier1Score,
		Tier2Score: tier2Score,
	}
}

// ─── Tier 1: Universal Rules ────────────────────────────────────────

func evaluateTier1(rawOutput string) (float64, []string) {
	var errors []string
	passed := 0
	total := 3

	// 1. Output must be valid JSON.
	trimmed := strings.TrimSpace(rawOutput)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		// Try to extract JSON from markdown code blocks.
		extracted := extractJSON(trimmed)
		if extracted != "" {
			if err2 := json.Unmarshal([]byte(extracted), &parsed); err2 != nil {
				errors = append(errors, fmt.Sprintf("T1-JSON: output is not valid JSON: %v", err))
			} else {
				passed++
				errors = append(errors, "T1-MARKDOWN: output contained markdown wrapping (extracted JSON)")
			}
		} else {
			errors = append(errors, fmt.Sprintf("T1-JSON: output is not valid JSON: %v", err))
		}
	} else {
		passed++
	}

	// 2. No null values on top-level fields.
	if parsed != nil {
		hasNull := false
		for k, v := range parsed {
			if v == nil {
				hasNull = true
				errors = append(errors, fmt.Sprintf("T1-NULL: top-level field '%s' is null", k))
			}
		}
		if !hasNull {
			passed++
		}
	}

	// 3. No markdown/preamble leaked in.
	if !strings.Contains(trimmed, "```") && !strings.HasPrefix(trimmed, "Here") &&
		!strings.HasPrefix(trimmed, "Sure") && !strings.HasPrefix(trimmed, "I ") {
		passed++
	} else {
		errors = append(errors, "T1-PREAMBLE: output contains markdown or preamble text")
	}

	if total == 0 {
		return 1.0, errors
	}
	return float64(passed) / float64(total), errors
}

// ─── Tier 2: Inferred Rules ────────────────────────────────────────

func evaluateTier2(rawOutput string, rules []Rule) (float64, []string) {
	if len(rules) == 0 {
		return 1.0, nil // No rules = perfect score.
	}

	trimmed := strings.TrimSpace(rawOutput)
	extracted := extractJSON(trimmed)
	if extracted != "" {
		trimmed = extracted
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return 0.0, []string{"T2-PARSE: cannot parse output to apply rules"}
	}

	passed := 0
	var errors []string

	for _, rule := range rules {
		err := applyRule(parsed, rule)
		if err != nil {
			errors = append(errors, fmt.Sprintf("T2-%s: %s", rule.RuleID, err.Error()))
		} else {
			passed++
		}
	}

	return float64(passed) / float64(len(rules)), errors
}

// applyRule checks a single rule against the parsed JSON output.
func applyRule(data map[string]interface{}, rule Rule) error {
	// Resolve the field value using dot-notation path.
	val := resolveField(data, rule.FieldPath)

	switch rule.RuleType {
	case "presence":
		if val == nil {
			return fmt.Errorf("field '%s' is missing (required by rule)", rule.FieldPath)
		}

	case "equality":
		var expected interface{}
		if err := json.Unmarshal([]byte(rule.ValueJSON), &expected); err != nil {
			log.Printf("[eval] warning: cannot parse value_json for rule %s: %v", rule.RuleID, err)
			return nil // Skip malformed rules.
		}
		if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", expected) {
			return fmt.Errorf("field '%s' expected %v, got %v", rule.FieldPath, expected, val)
		}

	case "range":
		numVal, ok := toFloat64(val)
		if !ok {
			return fmt.Errorf("field '%s' is not numeric for range check", rule.FieldPath)
		}
		var rangeVal map[string]interface{}
		if err := json.Unmarshal([]byte(rule.ValueJSON), &rangeVal); err != nil {
			return nil
		}
		if minV, ok := rangeVal["min"]; ok {
			if minF, ok := toFloat64(minV); ok && numVal < minF {
				return fmt.Errorf("field '%s' value %v is below min %v", rule.FieldPath, numVal, minF)
			}
		}
		if maxV, ok := rangeVal["max"]; ok {
			if maxF, ok := toFloat64(maxV); ok && numVal > maxF {
				return fmt.Errorf("field '%s' value %v is above max %v", rule.FieldPath, numVal, maxF)
			}
		}

	case "enum":
		var allowedList []interface{}
		if err := json.Unmarshal([]byte(rule.ValueJSON), &allowedList); err != nil {
			return nil
		}
		strVal := fmt.Sprintf("%v", val)
		found := false
		for _, a := range allowedList {
			if fmt.Sprintf("%v", a) == strVal {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field '%s' value '%v' is not in allowed set %v", rule.FieldPath, val, allowedList)
		}

	case "conditional":
		// Conditional rules: check condition first, then apply the main check.
		var cond map[string]interface{}
		if err := json.Unmarshal([]byte(rule.ConditionJSON), &cond); err != nil {
			return nil
		}
		condField, _ := cond["field"].(string)
		condVal := resolveField(data, condField)
		expectedCondStr := fmt.Sprintf("%v", cond["value"])
		if fmt.Sprintf("%v", condVal) != expectedCondStr {
			return nil // Condition not met, skip rule.
		}
		// Condition met — check the main field.
		if val == nil {
			return fmt.Errorf("conditional: field '%s' is missing when condition is met", rule.FieldPath)
		}

	default:
		// Unknown rule type — pass silently.
	}

	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────

// resolveField navigates a dot-separated path in a nested map.
func resolveField(data map[string]interface{}, path string) interface{} {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}
	return current
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// extractJSON tries to pull a JSON object out of markdown code fences.
func extractJSON(s string) string {
	// Look for ```json ... ``` pattern.
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(s[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		start := idx + 3
		// Skip optional language tag on the same line.
		if nlIdx := strings.Index(s[start:], "\n"); nlIdx >= 0 {
			start += nlIdx + 1
		}
		end := strings.Index(s[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	return ""
}
