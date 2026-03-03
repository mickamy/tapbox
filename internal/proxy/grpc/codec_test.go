package grpc_test

import (
	"testing"

	"google.golang.org/grpc/mem"

	grpcproxy "github.com/mickamy/tapbox/internal/proxy/grpc"
)

func TestRawCodec_Name(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	codec := grpcproxy.RawCodec{}
	if name := codec.Name(); name != "proto" {
		t.Errorf("Name() = %q, want %q", name, "proto")
	}
}

func TestRawCodec_MarshalByteSlicePtr(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	codec := grpcproxy.RawCodec{}
	data := []byte("hello world")
	result, err := codec.Marshal(&data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := result.Materialize()
	if string(got) != "hello world" {
		t.Errorf("Marshal result = %q, want %q", got, "hello world")
	}
}

func TestRawCodec_MarshalByteSlice(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	codec := grpcproxy.RawCodec{}
	data := []byte("hello world")
	result, err := codec.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := result.Materialize()
	if string(got) != "hello world" {
		t.Errorf("Marshal result = %q, want %q", got, "hello world")
	}
}

func TestRawCodec_MarshalErrorForOtherTypes(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	codec := grpcproxy.RawCodec{}
	_, err := codec.Marshal("not bytes")
	if err == nil {
		t.Fatal("Marshal: expected error for unsupported type, got nil")
	}
}

func TestRawCodec_UnmarshalByteSlicePtr(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	codec := grpcproxy.RawCodec{}
	data := []byte("test data")
	buf := mem.SliceBuffer(data)
	bs := mem.BufferSlice{buf}

	var out []byte
	if err := codec.Unmarshal(bs, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(out) != "test data" {
		t.Errorf("Unmarshal result = %q, want %q", out, "test data")
	}
}

func TestRawCodec_UnmarshalErrorForOtherTypes(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	codec := grpcproxy.RawCodec{}
	data := []byte("test")
	buf := mem.SliceBuffer(data)
	bs := mem.BufferSlice{buf}

	var s string
	if err := codec.Unmarshal(bs, &s); err == nil {
		t.Fatal("Unmarshal: expected error for unsupported type, got nil")
	}
}
