package resp

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// encode encodes a Value to a string for easy assertion.
func encode(v Value) string {
	var buf bytes.Buffer
	if err := v.WriteTo(&buf); err != nil {
		panic(err)
	}
	return buf.String()
}

// decode reads a single Value from a string.
func decode(s string) (Value, error) {
	return NewReader(strings.NewReader(s)).ReadValue()
}

func TestSimpleString(t *testing.T) {
	v := SimpleString("OK")
	if got := encode(v); got != "+OK\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("+OK\r\n")
	if err != nil || v2.Str() != "OK" || v2.Type() != TypeSimpleString {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestError(t *testing.T) {
	v := Error("ERR something went wrong")
	if got := encode(v); got != "-ERR something went wrong\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("-ERR something went wrong\r\n")
	if err != nil || v2.Str() != "ERR something went wrong" || !v2.IsError() {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestInteger(t *testing.T) {
	for _, n := range []int64{0, 1, -1, 42, -9999} {
		v := Integer(n)
		encoded := encode(v)
		v2, err := decode(encoded)
		if err != nil || v2.Integer() != n || v2.Type() != TypeInteger {
			t.Fatalf("round-trip %d: %v %v", n, v2, err)
		}
	}
}

func TestBulkString(t *testing.T) {
	v := BulkString("hello")
	if got := encode(v); got != "$5\r\nhello\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("$5\r\nhello\r\n")
	if err != nil || v2.Str() != "hello" || v2.IsNull() {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestBulkStringEmpty(t *testing.T) {
	v := BulkString("")
	if got := encode(v); got != "$0\r\n\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("$0\r\n\r\n")
	if err != nil || v2.Str() != "" || v2.IsNull() {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestNullBulkString(t *testing.T) {
	v := NullBulkString()
	if got := encode(v); got != "$-1\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("$-1\r\n")
	if err != nil || !v2.IsNull() {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestArray(t *testing.T) {
	v := Array([]Value{BulkString("SET"), BulkString("key"), BulkString("val")})
	encoded := encode(v)
	if encoded != "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$3\r\nval\r\n" {
		t.Fatalf("encode: got %q", encoded)
	}
	v2, err := decode(encoded)
	if err != nil || len(v2.Elems()) != 3 {
		t.Fatalf("decode: %v %v", v2, err)
	}
	if v2.Elems()[0].Str() != "SET" {
		t.Fatalf("elem 0: %v", v2.Elems()[0])
	}
}

func TestEmptyArray(t *testing.T) {
	v := Array([]Value{})
	if got := encode(v); got != "*0\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("*0\r\n")
	if err != nil || len(v2.Elems()) != 0 || v2.IsNull() {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestNullArray(t *testing.T) {
	v := NullArray()
	if got := encode(v); got != "*-1\r\n" {
		t.Fatalf("encode: got %q", got)
	}
	v2, err := decode("*-1\r\n")
	if err != nil || !v2.IsNull() {
		t.Fatalf("decode: %v %v", v2, err)
	}
}

func TestNestedArray(t *testing.T) {
	inner := Array([]Value{Integer(1), Integer(2)})
	outer := Array([]Value{BulkString("x"), inner})
	encoded := encode(outer)
	v2, err := decode(encoded)
	if err != nil || len(v2.Elems()) != 2 {
		t.Fatalf("decode: %v %v", v2, err)
	}
	if v2.Elems()[1].Type() != TypeArray || len(v2.Elems()[1].Elems()) != 2 {
		t.Fatalf("inner array: %v", v2.Elems()[1])
	}
}

func TestPipelining(t *testing.T) {
	// Two commands concatenated in one buffer — simulates client pipelining.
	data := "*1\r\n$4\r\nPING\r\n*3\r\n$3\r\nSET\r\n$1\r\na\r\n$1\r\nb\r\n"
	rd := NewReader(strings.NewReader(data))

	v1, err := rd.ReadValue()
	if err != nil || len(v1.Elems()) != 1 || v1.Elems()[0].Str() != "PING" {
		t.Fatalf("first command: %v %v", v1, err)
	}

	v2, err := rd.ReadValue()
	if err != nil || len(v2.Elems()) != 3 || v2.Elems()[0].Str() != "SET" {
		t.Fatalf("second command: %v %v", v2, err)
	}
}

func TestEOF(t *testing.T) {
	_, err := decode("")
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestTruncatedBulkString(t *testing.T) {
	_, err := decode("$5\r\nhel")
	if err == nil {
		t.Fatal("expected error for truncated bulk string")
	}
}

func TestInvalidIntegerLine(t *testing.T) {
	_, err := decode(":abc\r\n")
	if err == nil {
		t.Fatal("expected error for non-integer")
	}
}

func TestInlineCommand(t *testing.T) {
	v, err := decode("PING\r\n")
	if err != nil || len(v.Elems()) != 1 || v.Elems()[0].Str() != "PING" {
		t.Fatalf("inline PING: %v %v", v, err)
	}

	v2, err := decode("SET key value\r\n")
	if err != nil || len(v2.Elems()) != 3 {
		t.Fatalf("inline SET: %v %v", v2, err)
	}
}

func TestBulkStringWithBinaryContent(t *testing.T) {
	s := "hello\r\nworld"
	v := BulkString(s)
	encoded := encode(v)
	v2, err := decode(encoded)
	if err != nil || v2.Str() != s {
		t.Fatalf("binary content round-trip: %v %v", v2, err)
	}
}
