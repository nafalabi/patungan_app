package utils

import (
	"strings"

	"patungan_app_echo/internal/models"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// FormatRupiah formats a float64 to Indonesian Rupiah format (e.g., Rp 73.000,00)
func FormatRupiah(amount float64) string {
	p := message.NewPrinter(language.Indonesian)
	return p.Sprintf("Rp %.2f", amount)
}

// FormatRupiahSimple formats a float64 to Indonesian Rupiah format without decimals (e.g., Rp 73.000)
func FormatRupiahSimple(amount float64) string {
	p := message.NewPrinter(language.Indonesian)
	return p.Sprintf("Rp %.0f", amount)
}

// SafeInitial safely returns the first uppercase character of a string, or a fallback if empty
func SafeInitial(name string, fallback ...string) string {
	defaultFallback := "—"
	if len(fallback) > 0 && fallback[0] != "" {
		defaultFallback = fallback[0]
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return defaultFallback
	}
	runes := []rune(trimmed)
	if len(runes) == 0 {
		return defaultFallback
	}
	return strings.ToUpper(string(runes[0]))
}

// SafeInitials returns up to two initials (e.g. "John Doe" -> "JD", "John" -> "JO" or "J"), handling empty strings and irregular spaces
func SafeInitials(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return "—"
	}
	if len(fields) == 1 {
		runes := []rune(fields[0])
		if len(runes) == 1 {
			return strings.ToUpper(string(runes[0]))
		}
		if len(runes) >= 2 {
			return strings.ToUpper(string(runes[:2]))
		}
		return "—"
	}
	r1 := []rune(fields[0])
	r2 := []rune(fields[len(fields)-1])
	if len(r1) > 0 && len(r2) > 0 {
		return strings.ToUpper(string(r1[0]) + string(r2[0]))
	}
	if len(r1) > 0 {
		return strings.ToUpper(string(r1[0]))
	}
	return "—"
}

// GetTotalPortions calculates total portions from participants (matching billing logic)
func GetTotalPortions(participants []models.PlanParticipant) int {
	total := 0
	for _, p := range participants {
		total += p.Portion
	}
	return total
}


