package operators

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// A string constant read as the kind of the value beside it.
//
// A template has the literals of RFC 9535 - a string, a number, true, false,
// null - and no others: a point in time, a date or a UUID in a template is a
// string, `@.created_at > '2026-09-01'`. PostgreSQL reads an untyped parameter
// by the type of the column beside it, and so does the evaluator: a string
// compared with a value of a kind that has no literal of its own is read as
// that kind first, by the reader registered for the kind. What is read is a
// subset of what the server reads, so that nothing the evaluator accepts
// fails on the server.
//
// The default registry reads a time.Time and a uuid.UUID. A kind of the
// domain's own - the type it holds a date in - is read as the domain says,
// RegisterReader, beside the operators it registers for the type. A string
// beside a number or a boolean stays a string: those have literals, and a
// string there is the author's choice. See ADR-0015 of the reference.

// Reader reads a text as a value of one kind, or says why it is not one.
type Reader func(text string) (any, error)

// RegisterReader registers fn as what a string beside a T is read with.
func RegisterReader[T any](reg *OperatorRegistry, fn func(string) (T, error)) {
	var zero T
	reg.readers[reflect.TypeOf(zero)] = func(text string) (any, error) {
		return fn(text)
	}
}

// Read returns text read as the kind of other, or text as it is where no
// reader is registered for the kind. A pointer is what it points at, as it
// is to the operators.
func (r *OperatorRegistry) Read(text string, other any) (any, error) {
	if read, ok := r.readers[reflect.TypeOf(Indirect(other))]; ok {
		return read(text)
	}
	return text, nil
}

// ISO 8601: a date, or a date followed by T or a space and a time of hours and
// minutes, seconds, a fraction of up to six digits, then nothing, Z, or an
// offset. The server reads more - `20260901`, `Sep 1 2026`, `yesterday` - and
// those are an error here: loud, and never the other way round.
var pointInTime = regexp.MustCompile(
	`^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,6}))?)?(Z|[+-]\d{2}:\d{2})?)?$`,
)

// The canonical form, in either case. The server takes it without hyphens and
// in braces as well; here those are an error.
var canonicalUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// ReadPointInTime reads text as a point in time: a date, which is its
// midnight, or a date and a time, with its offset, in UTC without one. The
// time is in the zone of its offset, so its Date() is the date as spelled -
// what the server takes for a `date` column - and a reader of the domain's
// date type is built of this one.
func ReadPointInTime(text string) (time.Time, error) {
	notOne := func(why string) (time.Time, error) {
		return time.Time{}, fmt.Errorf("'%s' is not a point in time: %s", text, why)
	}
	m := pointInTime.FindStringSubmatch(text)
	if m == nil {
		return notOne("YYYY-MM-DD, or that with THH:MM[:SS[.ffffff]] and Z or an offset")
	}
	number := func(digits string) int {
		n, _ := strconv.Atoi(digits) // digits by the pattern; "" is 0
		return n
	}
	year, month, day := number(m[1]), number(m[2]), number(m[3])
	hour, minute, second := number(m[4]), number(m[5]), number(m[6])
	nanos := number((m[7] + "000000000")[:9])
	zone := time.UTC
	if offset := m[8]; offset != "" && offset != "Z" {
		seconds := (number(offset[1:3])*60 + number(offset[4:6])) * 60
		if offset[0] == '-' {
			seconds = -seconds
		}
		zone = time.FixedZone(offset, seconds)
	}
	moment := time.Date(year, time.Month(month), day, hour, minute, second, nanos, zone)
	// time.Date carries a thirteenth month into the next year; the server
	// refuses it, and so does this.
	if moment.Year() != year || int(moment.Month()) != month || moment.Day() != day ||
		moment.Hour() != hour || moment.Minute() != minute || moment.Second() != second {
		return notOne("no such date or time")
	}
	return moment, nil
}

func readUUID(text string) (uuid.UUID, error) {
	if !canonicalUUID.MatchString(text) {
		return uuid.UUID{}, fmt.Errorf("'%s' is not a UUID: 8-4-4-4-12 hexadecimal digits", text)
	}
	return uuid.Parse(text)
}
