package sql_test

import (
	"testing"

	"github.com/mickamy/tapbox/internal/proxy/sql"
)

func TestParseSQLComment_ValidTraceparent(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	query := "SELECT 1 /*traceparent='00-abcdef0123456789abcdef0123456789-0123456789abcdef-01'*/"
	result, ok := sql.ParseSQLComment(query)
	if !ok {
		t.Fatal("expected ok=true for valid traceparent")
	}
	if result.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("TraceID = %q, want %q", result.TraceID, "abcdef0123456789abcdef0123456789")
	}
	if result.ParentID != "0123456789abcdef" {
		t.Errorf("ParentID = %q, want %q", result.ParentID, "0123456789abcdef")
	}
}

func TestParseSQLComment_MultipleKeys(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	query := "SELECT 1 /*db_driver='pgx',traceparent='00-abcdef0123456789abcdef0123456789-0123456789abcdef-01'*/"
	result, ok := sql.ParseSQLComment(query)
	if !ok {
		t.Fatal("expected ok=true for traceparent among multiple keys")
	}
	if result.TraceID != "abcdef0123456789abcdef0123456789" {
		t.Errorf("TraceID = %q, want %q", result.TraceID, "abcdef0123456789abcdef0123456789")
	}
	if result.ParentID != "0123456789abcdef" {
		t.Errorf("ParentID = %q, want %q", result.ParentID, "0123456789abcdef")
	}
}

func TestParseSQLComment_NoComment(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, ok := sql.ParseSQLComment("SELECT 1")
	if ok {
		t.Error("expected ok=false for query without comment")
	}
}

func TestParseSQLComment_NoTraceparentKey(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, ok := sql.ParseSQLComment("SELECT 1 /*db_driver='pgx'*/")
	if ok {
		t.Error("expected ok=false when traceparent key is absent")
	}
}

func TestParseSQLComment_InvalidTraceparentFormat(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, ok := sql.ParseSQLComment("SELECT 1 /*traceparent='invalid'*/")
	if ok {
		t.Error("expected ok=false for invalid traceparent value")
	}
}

func TestParseSQLComment_UnclosedComment(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, ok := sql.ParseSQLComment("SELECT 1 /*traceparent='00-abcdef0123456789abcdef0123456789-0123456789abcdef-01'")
	if ok {
		t.Error("expected ok=false for unclosed comment")
	}
}

func TestStripSQLComment_TrailingComment(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	got := sql.StripSQLComment("SELECT id FROM foo /*traceparent='00-abc-def-01'*/")
	want := "SELECT id FROM foo"
	if got != want {
		t.Errorf("StripSQLComment = %q, want %q", got, want)
	}
}

func TestStripSQLComment_MiddleComment(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	query := "SELECT /* hint */ id FROM foo"
	got := sql.StripSQLComment(query)
	if got != query {
		t.Errorf("StripSQLComment = %q, want %q (unchanged)", got, query)
	}
}

func TestStripSQLComment_NoComment(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	query := "SELECT id FROM foo"
	got := sql.StripSQLComment(query)
	if got != query {
		t.Errorf("StripSQLComment = %q, want %q (unchanged)", got, query)
	}
}
