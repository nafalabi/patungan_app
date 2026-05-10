package utils

import (
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
