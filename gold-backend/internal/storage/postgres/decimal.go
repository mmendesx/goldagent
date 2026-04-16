package postgres

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// decimalToString converts a decimal.Decimal to its string representation for
// use with $N::numeric SQL casts in parameterized queries.
func decimalToString(d decimal.Decimal) string {
	return d.String()
}

// decimalToNumeric converts a decimal.Decimal to a pgtype.Numeric for use with
// pgx CopyFrom, which requires properly typed values for binary COPY protocol.
func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	// ScanScientific parses a decimal string and marks the value valid.
	_ = n.ScanScientific(d.String())
	return n
}
