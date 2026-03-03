package sql_test

import (
	"testing"

	"github.com/mickamy/tapbox/internal/proxy/sql"
)

func TestParseSQLComment_ValidTraceparent(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	query := "SELECT 1 /*traceparent='00-abcdef0123456789abcdef0123456789-0123456789abcdef-01'*/"
	result, status := sql.ParseSQLComment(query)
	if status != sql.CommentOK {
		t.Fatalf("status = %d, want CommentOK", status)
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
	result, status := sql.ParseSQLComment(query)
	if status != sql.CommentOK {
		t.Fatalf("status = %d, want CommentOK", status)
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

	_, status := sql.ParseSQLComment("SELECT 1")
	if status != sql.CommentAbsent {
		t.Errorf("status = %d, want CommentAbsent", status)
	}
}

func TestParseSQLComment_NoTraceparentKey(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, status := sql.ParseSQLComment("SELECT 1 /*db_driver='pgx'*/")
	if status != sql.CommentAbsent {
		t.Errorf("status = %d, want CommentAbsent when traceparent key is absent", status)
	}
}

func TestParseSQLComment_InvalidTraceparentFormat(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, status := sql.ParseSQLComment("SELECT 1 /*traceparent='invalid'*/")
	if status != sql.CommentInvalid {
		t.Errorf("status = %d, want CommentInvalid for malformed traceparent", status)
	}
}

func TestParseSQLComment_UnclosedComment(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, status := sql.ParseSQLComment("SELECT 1 /*traceparent='00-abcdef0123456789abcdef0123456789-0123456789abcdef-01'")
	if status != sql.CommentAbsent {
		t.Errorf("status = %d, want CommentAbsent for unclosed comment", status)
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
