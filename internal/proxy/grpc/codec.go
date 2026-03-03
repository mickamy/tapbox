package grpc

import (
	"fmt"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/proto"
)

func init() {
	encoding.RegisterCodecV2(RawCodec{})
}

// RawCodec is a gRPC codec that handles both raw []byte (for transparent proxying)
// and proto.Message (for normal gRPC clients). It is registered as "proto" so that
// incoming requests with the default content-type are handled correctly.
type RawCodec struct{}

func (RawCodec) Marshal(v any) (mem.BufferSlice, error) {
	switch b := v.(type) {
	case *[]byte:
		return mem.BufferSlice{mem.SliceBuffer(*b)}, nil
	case []byte:
		return mem.BufferSlice{mem.SliceBuffer(b)}, nil
	default:
		if m, ok := v.(proto.Message); ok {
			data, err := proto.Marshal(m)
			if err != nil {
				return nil, fmt.Errorf("raw codec marshal proto: %w", err)
			}
			return mem.BufferSlice{mem.SliceBuffer(data)}, nil
		}
		return nil, fmt.Errorf("raw codec: unsupported marshal type %T", v)
	}
}

func (RawCodec) Unmarshal(data mem.BufferSlice, v any) error {
	buf := data.Materialize()
	switch b := v.(type) {
	case *[]byte:
		*b = buf
		return nil
	default:
		if m, ok := v.(proto.Message); ok {
			if err := proto.Unmarshal(buf, m); err != nil {
				return fmt.Errorf("raw codec unmarshal proto: %w", err)
			}
			return nil
		}
		return fmt.Errorf("raw codec: unsupported unmarshal type %T", v)
	}
}

// Name returns "proto" to override the default codec so that the gRPC proxy's
// UnknownServiceHandler can receive and send raw bytes on the server stream.
func (RawCodec) Name() string { return "proto" }
