// Package resp implements the RESP2 (Redis Serialization Protocol) encoder and decoder.
// It is dependency-free and has no knowledge of commands or storage.
package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Type constants for RESP2 wire types.
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

// Value represents a RESP2 value.
type Value struct {
	typ     byte
	str     string
	integer int64
	elems   []Value
	isNull  bool
}

// SimpleString returns a simple string Value (+OK, +PONG, etc.).
func SimpleString(s string) Value { return Value{typ: TypeSimpleString, str: s} }

// Error returns an error Value.
func Error(s string) Value { return Value{typ: TypeError, str: s} }

// Integer returns an integer Value.
func Integer(n int64) Value { return Value{typ: TypeInteger, integer: n} }

// BulkString returns a bulk string Value.
func BulkString(s string) Value { return Value{typ: TypeBulkString, str: s} }

// NullBulkString returns a null bulk string ($-1\r\n).
func NullBulkString() Value { return Value{typ: TypeBulkString, isNull: true} }

// Array returns an array Value.
func Array(elems []Value) Value { return Value{typ: TypeArray, elems: elems} }

// NullArray returns a null array (*-1\r\n).
func NullArray() Value { return Value{typ: TypeArray, isNull: true} }

func (v Value) Type() byte     { return v.typ }
func (v Value) Str() string    { return v.str }
func (v Value) Integer() int64 { return v.integer }
func (v Value) Elems() []Value { return v.elems }
func (v Value) IsNull() bool   { return v.isNull }

// IsError returns true when the value is a RESP error.
func (v Value) IsError() bool { return v.typ == TypeError }

// WriteTo encodes the value as RESP2 bytes into w.
func (v Value) WriteTo(w io.Writer) error {
	bw, ok := w.(*bufio.Writer)
	if !ok {
		bw = bufio.NewWriter(w)
	}
	if err := writeValue(bw, v); err != nil {
		return err
	}
	return bw.Flush()
}

func writeValue(w *bufio.Writer, v Value) error {
	switch v.typ {
	case TypeSimpleString:
		_, err := fmt.Fprintf(w, "+%s\r\n", v.str)
		return err
	case TypeError:
		_, err := fmt.Fprintf(w, "-%s\r\n", v.str)
		return err
	case TypeInteger:
		_, err := fmt.Fprintf(w, ":%d\r\n", v.integer)
		return err
	case TypeBulkString:
		if v.isNull {
			_, err := w.WriteString("$-1\r\n")
			return err
		}
		if _, err := fmt.Fprintf(w, "$%d\r\n", len(v.str)); err != nil {
			return err
		}
		if _, err := w.WriteString(v.str); err != nil {
			return err
		}
		_, err := w.WriteString("\r\n")
		return err
	case TypeArray:
		if v.isNull {
			_, err := w.WriteString("*-1\r\n")
			return err
		}
		if _, err := fmt.Fprintf(w, "*%d\r\n", len(v.elems)); err != nil {
			return err
		}
		for _, elem := range v.elems {
			if err := writeValue(w, elem); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown value type %q", v.typ)
	}
}

// Reader decodes RESP2 values from a buffered reader.
// A single Reader must not be used from multiple goroutines concurrently.
type Reader struct {
	r *bufio.Reader
}

// NewReader wraps r in a RESP2 Reader.
func NewReader(r io.Reader) *Reader {
	if br, ok := r.(*bufio.Reader); ok {
		return &Reader{r: br}
	}
	return &Reader{r: bufio.NewReader(r)}
}

// ReadValue reads the next RESP2 value from the stream.
// Returns io.EOF when the connection is closed cleanly.
func (rd *Reader) ReadValue() (Value, error) {
	return rd.readValue()
}

func (rd *Reader) readValue() (Value, error) {
	b, err := rd.r.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch b {
	case TypeSimpleString:
		line, err := rd.readLine()
		if err != nil {
			return Value{}, err
		}
		return SimpleString(line), nil

	case TypeError:
		line, err := rd.readLine()
		if err != nil {
			return Value{}, err
		}
		return Error(line), nil

	case TypeInteger:
		line, err := rd.readLine()
		if err != nil {
			return Value{}, err
		}
		n, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("resp: invalid integer %q", line)
		}
		return Integer(n), nil

	case TypeBulkString:
		return rd.readBulkString()

	case TypeArray:
		return rd.readArray()

	default:
		// Inline command: unread the byte and read the whole line
		if err := rd.r.UnreadByte(); err != nil {
			return Value{}, err
		}
		return rd.readInline()
	}
}

func (rd *Reader) readLine() (string, error) {
	line, err := rd.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", errors.New("resp: missing CRLF")
	}
	return line[:len(line)-2], nil
}

func (rd *Reader) readBulkString() (Value, error) {
	line, err := rd.readLine()
	if err != nil {
		return Value{}, err
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		return Value{}, fmt.Errorf("resp: invalid bulk string length %q", line)
	}
	if n < 0 {
		return NullBulkString(), nil
	}

	buf := make([]byte, n+2) // +2 for trailing CRLF
	if _, err := io.ReadFull(rd.r, buf); err != nil {
		return Value{}, err
	}
	if buf[n] != '\r' || buf[n+1] != '\n' {
		return Value{}, errors.New("resp: missing CRLF after bulk string")
	}
	return BulkString(string(buf[:n])), nil
}

func (rd *Reader) readArray() (Value, error) {
	line, err := rd.readLine()
	if err != nil {
		return Value{}, err
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		return Value{}, fmt.Errorf("resp: invalid array length %q", line)
	}
	if n < 0 {
		return NullArray(), nil
	}

	elems := make([]Value, n)
	for i := range elems {
		v, err := rd.readValue()
		if err != nil {
			return Value{}, err
		}
		elems[i] = v
	}
	return Array(elems), nil
}

// readInline handles the inline command format (e.g. "PING\r\n" typed by hand).
// It converts the line into an array of bulk strings.
func (rd *Reader) readInline() (Value, error) {
	line, err := rd.readLine()
	if err != nil {
		return Value{}, err
	}
	parts := splitInline(line)
	elems := make([]Value, len(parts))
	for i, p := range parts {
		elems[i] = BulkString(p)
	}
	return Array(elems), nil
}

// splitInline splits an inline command line on spaces, respecting no quoting.
func splitInline(line string) []string {
	var parts []string
	start := -1
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			if start >= 0 {
				parts = append(parts, line[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		parts = append(parts, line[start:])
	}
	return parts
}
