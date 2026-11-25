// Package report renders the monthly summary PDF without external deps.
package report

import (
	"fmt"
	"strings"

	"meteran/internal/domain"
	"meteran/internal/store"
)

// SummaryPDF returns a single-page A4 report (FR-71).
func SummaryPDF(sum domain.Summary, rows []store.Reading, from, to string) []byte {
	lines := []string{
		"Laporan Meteran Prabayar",
		"Periode: " + display(from) + " s/d " + display(to),
		"Dibuat: " + sum.ComputedAt,
		"",
		fmt.Sprintf("Sisa kWh terakhir: %s", f2(latestValue(sum))),
		fmt.Sprintf("Pemakaian hari ini: %s kWh", f2p(sum.TodayKwh)),
		fmt.Sprintf("Rata-rata %d hari: %s kWh/hari", sum.WindowDays, f2p(sum.AvgKwhPerDay)),
		fmt.Sprintf("Prediksi sentuh buffer: %s", strp(sum.DepletionAt)),
		fmt.Sprintf("Harga efektif per kWh: %s", f2p(sum.EffectivePricePerKwh)),
		"",
		"10 pembacaan terakhir (tanpa baris top-up):",
	}
	shown := 0
	for i := len(rows) - 1; i >= 0 && shown < 10; i-- {
		row := rows[i]
		if row.RechargeAmount != nil && *row.RechargeAmount > 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s  %s kWh", row.Timestamp, f2(row.Reading)))
		shown++
	}
	if shown == 0 {
		lines = append(lines, "(belum ada pembacaan)")
	}
	return build(lines)
}

func latestValue(s domain.Summary) float64 {
	if s.LatestReading == nil {
		return 0
	}
	return s.LatestReading.Reading
}

func display(iso string) string {
	if iso == "" {
		return "awal"
	}
	if t, err := store.ParseTime(iso); err == nil {
		return t.In(store.WIB).Format("02 Jan 2006")
	}
	return iso
}

func f2(v float64) string { return fmt.Sprintf("%.2f", v) }

func f2p(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return f2(*v)
}

func strp(v *string) string {
	if v == nil {
		return "N/A"
	}
	return *v
}

// build assembles a minimal PDF 1.4 file with one page of Helvetica text.
func build(lines []string) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 11 Tf 50 792 Td 16 TL\n")
	for _, line := range lines {
		content.WriteString("(" + escape(line) + ") Tj T*\n")
	}
	content.WriteString("ET")
	stream := content.String()

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(out.String())
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "(", `\(`)
	return strings.ReplaceAll(s, ")", `\)`)
}