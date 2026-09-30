// Package calendar is the type a domain under test holds a date in: one the
// specification package does not know, and does not choose. The tests of the
// evaluator, of the compiler and of the templates hold a `date` column in it,
// so that a string beside it is read as the domain registers - a `date`
// column held as a time.Time midnight agrees with the server on a string that
// is a date alone, and parts from it in silence on `< '2026-09-02T12:00:00Z'`:
// the server takes the date of it, and a midnight is less than the noon.
package calendar

import (
	"cmp"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// Date is a calendar date, without a time or a zone.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// New is the date of the day.
func New(year int, month time.Month, day int) Date {
	return Date{year, month, day}
}

// Value is the date as the storage's driver writes it: a `date`.
func (d Date) Value() (driver.Value, error) {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day), nil
}

// Read is the domain's reader of a string beside a Date: the date of the
// point in time as spelled, which is what the server takes for a `date`.
func Read(text string) (Date, error) {
	moment, err := operators.ReadPointInTime(text)
	if err != nil {
		return Date{}, err
	}
	year, month, day := moment.Date()
	return Date{year, month, day}, nil
}

// Compare orders two dates.
func Compare(a, b Date) int {
	ordinal := func(d Date) int { return d.Year*10000 + int(d.Month)*100 + d.Day }
	return cmp.Compare(ordinal(a), ordinal(b))
}

// Registry is the default registry with what the domain registers for its
// Date: the order of it, and the reader of a string beside it.
func Registry() *operators.OperatorRegistry {
	reg := operators.NewDefaultRegistry()
	operators.RegisterOrder(reg, Compare)
	operators.RegisterReader(reg, Read)
	return reg
}
