// Copyright (c) 2014 José Carlos Nieto, https://menteslibres.net/xiam
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

// Package resp provides methods for encoding and decoding RESP messages. RESP
// is the serialization protocol that redis uses
// (http://redis.io/topics/protocolV).
package resp

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

var endOfLine = []byte{'\r', '\n'}

var (
	typeErr     = reflect.TypeOf(errors.New(""))
	typeMessage = reflect.TypeOf(Message{})
)

const (
	// Bulk Strings are used in order to represent a single binary safe string up
	// to 512 MB in length.
	bulkMessageMaxLength = 512 * 1024
)

func byteToTypeName(c byte) string {
	switch c {
	case StringHeader:
		return `status`
	case ErrorHeader:
		return `error`
	case IntegerHeader:
		return `integer`
	case BulkHeader:
		return `bulk`
	case ArrayHeader:
		return `array`
	}
	return `unknown`
}

// Marshal returns the RESP encoding of v. At this moment, it only works with
// string, int, []byte, nil and []interface{} types.
func Marshal(v interface{}) ([]byte, error) {

	switch t := v.(type) {
	case string:
		// If the user sends a string, we convert it to byte to make it binary
		// safe.
		v = []byte(t)
	}

	e := NewEncoder(nil)

	if err := e.Encode(v); err != nil {
		return nil, err
	}

	return e.buf, nil
}

// Unmarshal parses the RESP-encoded data and stores the result in the value
// pointed to by v. At this moment, it only works with string, int, []byte and
// []interface{} types.
func Unmarshal(data []byte, v interface{}) error {
	var err error

	if v == nil {
		return ErrExpectingPointer
	}

	r := bytes.NewReader(data)
	d := NewDecoder(r)

	if err = d.Decode(v); err != nil {
		return err
	}

	return nil
}

func redisMessageToType(dst reflect.Value, out *Message) error {

	if dst.Type() == typeMessage {
		dst.Set(reflect.ValueOf(*out))
		return nil
	}

	if out.IsNil {
		dst.Set(reflect.Zero(dst.Type()))
		return ErrMessageIsNil
	}

	dstKind := dst.Type().Kind()

	// User wants a conversion.
	switch out.Type {
	case StringHeader:
		switch dstKind {
		// string -> string.
		case reflect.String:
			dst.Set(reflect.ValueOf(out.Status))
			return nil
		case reflect.Interface:
			dst.Set(reflect.ValueOf(out))
			return nil
		}
	case ErrorHeader:
		switch dstKind {
		// error -> string
		case reflect.String:
			dst.Set(reflect.ValueOf(out.Error.Error()))
			return nil
		// error -> serror
		case typeErr.Kind():
			dst.Set(reflect.ValueOf(out.Error))
			return nil
		case reflect.Interface:
			dst.Set(reflect.ValueOf(out))
			return nil
		}
	case IntegerHeader:
		switch dstKind {
		case reflect.Int:
			// integer -> integer.
			dst.Set(reflect.ValueOf(int(out.Integer)))
			return nil
		case reflect.Int64:
			// integer -> integer64.
			dst.Set(reflect.ValueOf(out.Integer))
			return nil
		case reflect.String:
			// integer -> string.
			dst.Set(reflect.ValueOf(strconv.FormatInt(out.Integer, 10)))
			return nil
		case reflect.Bool:
			// integer -> bool.
			if out.Integer == 0 {
				dst.Set(reflect.ValueOf(false))
			} else {
				dst.Set(reflect.ValueOf(true))
			}
			return nil
		case reflect.Interface:
			dst.Set(reflect.ValueOf(out))
			return nil
		}
	case BulkHeader:
		switch dstKind {
		case reflect.String:
			// []byte -> string
			dst.Set(reflect.ValueOf(string(out.Bytes)))
			return nil
		case reflect.Slice:
			// []byte -> []byte
			dst.Set(reflect.ValueOf(out.Bytes))
			return nil
		case reflect.Int:
			// []byte -> int
			n, _ := strconv.Atoi(string(out.Bytes))
			dst.Set(reflect.ValueOf(n))
			return nil
		case reflect.Int64:
			// []byte -> int64
			n, _ := strconv.Atoi(string(out.Bytes))
			dst.Set(reflect.ValueOf(int64(n)))
			return nil
		case reflect.Interface:
			dst.Set(reflect.ValueOf(out))
			return nil
		}
	case ArrayHeader:
		switch dstKind {
		// slice -> interface
		case reflect.Interface:
			var err error
			var elements reflect.Value
			total := len(out.Array)

			elements = reflect.MakeSlice(reflect.TypeOf([]interface{}{}), total, total)

			for i := 0; i < total; i++ {
				if err = redisMessageToType(elements.Index(i), out.Array[i]); err != nil {
					if err != ErrMessageIsNil {
						return err
					}
				}
			}

			dst.Set(elements)

			return nil
		// slice -> slice
		case reflect.Slice:
			var err error
			var elements reflect.Value
			total := len(out.Array)

			elements = reflect.MakeSlice(dst.Type(), total, total)

			for i := 0; i < total; i++ {
				if err = redisMessageToType(elements.Index(i), out.Array[i]); err != nil {
					if err != ErrMessageIsNil {
						return err
					}
				}
			}

			dst.Set(elements)

			return nil
		}
	}

	return fmt.Errorf(ErrUnsupportedConversion.Error(), byteToTypeName(out.Type), dstKind)
}
