// Copyright (c) 2019 Andy Pan
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package gnet

import (
	"encoding/binary"
	"fmt"
)

type (
	// ICodec is the interface of gnet codec.
	ICodec interface {
		// Encode encodes frames upon server responses into TCP stream.
		Encode(c Conn, buf []byte) ([]byte, error)
		// Decode decodes frames from TCP stream via specific implementation.
		Decode(c Conn) ([]byte, error)
	}

	// LengthFieldBasedFrameCodec is the refactoring from
	// https://github.com/smallnest/goframe/blob/master/length_field_based_frameconn.go, licensed by Apache License 2.0.
	// It encodes/decodes frames into/from TCP stream with value of the length field in the message.
	LengthFieldBasedFrameCodec struct {
		encoderConfig EncoderConfig
		decoderConfig DecoderConfig
	}
)

// NewLengthFieldBasedFrameCodec instantiates and returns a codec based on the length field.
// It is the go implementation of netty LengthFieldBasedFrameecoder and LengthFieldPrepender.
// you can see javadoc of them to learn more details.
func NewLengthFieldBasedFrameCodec(ec EncoderConfig, dc DecoderConfig) *LengthFieldBasedFrameCodec {
	return &LengthFieldBasedFrameCodec{encoderConfig: ec, decoderConfig: dc}
}

// EncoderConfig config for encoder.
type EncoderConfig struct {
	// ByteOrder is the ByteOrder of the length field.
	ByteOrder binary.ByteOrder
	// LengthFieldLength is the length of the length field.
	LengthFieldLength int
	// LengthAdjustment is the compensation value to add to the value of the length field
	// LengthAdjustment int
	// LengthIncludesLengthFieldLength is true, the length of the prepended length field is added to the value of
	// the prepended length field
	// LengthIncludesLengthFieldLength bool
}

// DecoderConfig config for decoder.
type DecoderConfig struct {
	// ByteOrder is the ByteOrder of the length field.
	ByteOrder binary.ByteOrder
	// LengthFieldOffset is the offset of the length field
	// LengthFieldOffset int
	// LengthFieldLength is the length of the length field
	LengthFieldLength int
	// LengthAdjustment is the compensation value to add to the value of the length field
	// LengthAdjustment int
	// InitialBytesToStrip is the number of first bytes to strip out from the decoded frame
	// InitialBytesToStrip int
}

// Encode ...
func (cc *LengthFieldBasedFrameCodec) Encode(c Conn, buf []byte) (out []byte, err error) {
	length := len(buf)
	offset := cc.encoderConfig.LengthFieldLength
	out = make([]byte, offset+length)
	switch offset {
	case 1:
		if length >= 256 {
			return nil, fmt.Errorf("length does not fit into a byte: %d", length)
		}
		out[0] = byte(length)
	case 2:
		if length >= 65536 {
			return nil, fmt.Errorf("length does not fit into a short integer: %d", length)
		}
		cc.encoderConfig.ByteOrder.PutUint16(out, uint16(length))
	case 3:
		if length >= 16777216 {
			return nil, fmt.Errorf("length does not fit into a medium integer: %d", length)
		}
		writeUint24(cc.encoderConfig.ByteOrder, length, out)
	case 4:
		cc.encoderConfig.ByteOrder.PutUint32(out, uint32(length))
	}

	copy(out[offset:], buf)
	// out = append(out, buf...)

	return
}

// Decode ...
func (cc *LengthFieldBasedFrameCodec) Decode(c Conn) ([]byte, error) {
	var (
		in  []byte
		err error
	)

	in, err = c.Peek(cc.decoderConfig.LengthFieldLength)
	if err != nil || len(in) < cc.decoderConfig.LengthFieldLength {
		return nil, err
	}

	frameLength := cc.getFrameLength(in)
	// real message length
	msgLength := int(frameLength) + int(cc.decoderConfig.LengthFieldLength)
	// 10MB: 不处理，过一段时间之后会自动断线
	if msgLength <= 0 || msgLength >= 10485760 {
		return nil, nil
	}

	in, err = c.Peek(msgLength)
	if err != nil || len(in) < msgLength {
		return nil, err
	}

	fullMessage := make([]byte, int(frameLength))
	copy(fullMessage, in[cc.decoderConfig.LengthFieldLength:])
	c.Discard(msgLength)

	return fullMessage, nil
}

func (cc *LengthFieldBasedFrameCodec) getFrameLength(in []byte) uint32 {
	switch cc.decoderConfig.LengthFieldLength {
	case 1:
		return uint32(in[0])
	case 2:
		return uint32(cc.decoderConfig.ByteOrder.Uint16(in))
	case 3:
		return uint32(readUint24(cc.decoderConfig.ByteOrder, in))
	case 4:
		return uint32(cc.decoderConfig.ByteOrder.Uint32(in))
	}
	return uint32(cc.decoderConfig.ByteOrder.Uint32(in))
}

func readUint24(byteOrder binary.ByteOrder, b []byte) uint64 {
	_ = b[2]
	if byteOrder == binary.LittleEndian {
		return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16
	}
	return uint64(b[2]) | uint64(b[1])<<8 | uint64(b[0])<<16
}

func writeUint24(byteOrder binary.ByteOrder, v int, out []byte) {
	if byteOrder == binary.LittleEndian {
		out[0] = byte(v)
		out[1] = byte(v >> 8)
		out[2] = byte(v >> 16)
	} else {
		out[2] = byte(v)
		out[1] = byte(v >> 8)
		out[0] = byte(v >> 16)
	}
}
