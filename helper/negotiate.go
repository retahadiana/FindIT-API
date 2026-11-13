package helper

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	FormatJSON = "json"
	FormatCSV  = "csv"
)

func NegotiateFormat(c *fiber.Ctx) (string, error) {
	accept := strings.TrimSpace(c.Get(fiber.HeaderAccept))
	if accept == "" || accept == "*/*" {
		return FormatJSON, nil
	}
	for _, part := range strings.Split(accept, ",") {
		media := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch media {
		case "application/json", "application/*", "*/*":
			return FormatJSON, nil
		case "text/csv":
			return FormatCSV, nil
		}
	}
	return "", NotAcceptable("format tidak didukung; gunakan application/json atau text/csv")
}

func SafeCSVField(value string) string {
	if value == "" {
		return ""
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

func SendCSV(c *fiber.Ctx, filename string, headers []string, rows [][]string) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return Internal(err)
	}
	for _, row := range rows {
		safe := make([]string, len(row))
		for i, col := range row {
			safe[i] = SafeCSVField(col)
		}
		if err := w.Write(safe); err != nil {
			return Internal(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return Internal(err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Status(fiber.StatusOK).Send(buf.Bytes())
}
