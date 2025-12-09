package utils

import "strings"

// NormalizePlate нормализует номерной знак к единому формату
// Удаляет пробелы, дефисы и приводит к верхнему регистру
func NormalizePlate(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ToUpper(normalized)
	return normalized
}
