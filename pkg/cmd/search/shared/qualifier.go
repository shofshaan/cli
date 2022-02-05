package shared

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// These regexs are not perfect and mostly used to give some input validation.
// They are more permissive than the server so we can provide early feedback
// to the user in most cases but there are some inputs that will pass the
// regexs and then be rejected by the server.
var rangeRE = regexp.MustCompile(`^(>|>=|<|<=|\*\.\.)?\d+(\.\.(\*|\d+))?$`)
var dateTime = `(\d|-|\+|:|T)+`
var dateTimeRangeRE = regexp.MustCompile(fmt.Sprintf(`^(>|>=|<|<=|\*\.\.)?%s(\.\.(\*|%s))?$`, dateTime, dateTime))

type validator func(string) error

type qualifier struct {
	key       string
	kind      string
	set       bool
	validator validator
	value     string
}

type parameter = qualifier

func NewQualifier(key, kind, value string, validator func(string) error) *qualifier {
	return &qualifier{
		key:       key,
		kind:      kind,
		validator: validator,
		value:     value,
	}
}

func NewParameter(key, kind, value string, validator func(string) error) *parameter {
	return &parameter{
		key:       key,
		kind:      kind,
		validator: validator,
		value:     value,
	}
}

func (q *qualifier) IsSet() bool {
	return q.set
}

func (q *qualifier) Key() string {
	return q.key
}

func (q *qualifier) Set(value string) error {
	if q.validator != nil {
		err := q.validator(value)
		if err != nil {
			return err
		}
	}
	q.set = true
	q.value = value
	return nil
}

func (q *qualifier) String() string {
	return q.value
}

func (q *qualifier) Type() string {
	return q.kind
}

// Validate that value is one of a list of options
func OptsValidator(opts []string) validator {
	return func(value string) error {
		if !isIncluded(value, opts) {
			return fmt.Errorf("%s is not included in [%s]", value, strings.Join(opts, ", "))
		}
		return nil
	}
}

// Validate that each value in comma seperated list matches a value in list of options
func MultiOptsValidator(opts []string) validator {
	return func(list string) error {
		values := strings.Split(list, ",")
		for _, value := range values {
			value = strings.TrimSpace(value)
			if !isIncluded(value, opts) {
				return fmt.Errorf("%q is not included in [%s]", value, strings.Join(opts, ", "))
			}
		}
		return nil
	}
}

func isIncluded(value string, opts []string) bool {
	for _, opt := range opts {
		if value == opt {
			return true
		}
	}
	return false
}

// Validate that value is a boolean
func BoolValidator() validator {
	return func(value string) error {
		_, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s is not a boolean value", value)
		}
		return nil
	}
}

// Validate that value is a correct range format
func RangeValidator() validator {
	return func(value string) error {
		if !rangeRE.MatchString(value) {
			return fmt.Errorf("%s is an invalid range format", value)
		}
		return nil
	}
}

// Validate that value is a correct date format
func DateValidator() validator {
	return func(value string) error {
		if !dateTimeRangeRE.MatchString(value) {
			return fmt.Errorf("%s is an invalid date format", value)
		}
		return nil
	}
}
