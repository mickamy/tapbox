package sql_test

import (
	"testing"
	"time"

	"github.com/mickamy/tapbox/internal/proxy/sql"
)

func TestCorrelator_CorrelateWithoutActive(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	c := sql.NewCorrelator(0)
	traceID, parentID := c.Correlate()

	if len(traceID) != 32 {
		t.Errorf("traceID length = %d, want 32 (new trace ID)", len(traceID))
	}
	if parentID != "" {
		t.Errorf("parentID = %q, want empty (no parent)", parentID)
	}
}

func TestCorrelator_SetActiveAndCorrelate(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	c := sql.NewCorrelator(0)
	c.SetActive("mytrace", "myspan")

	traceID, parentID := c.Correlate()
	if traceID != "mytrace" {
		t.Errorf("traceID = %q, want %q", traceID, "mytrace")
	}
	if parentID != "myspan" {
		t.Errorf("parentID = %q, want %q", parentID, "myspan")
	}
}

func TestCorrelator_OverwriteActive(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	c := sql.NewCorrelator(0)
	c.SetActive("trace1", "span1")
	c.SetActive("trace2", "span2")

	traceID, parentID := c.Correlate()
	if traceID != "trace2" {
		t.Errorf("traceID = %q, want %q (latest)", traceID, "trace2")
	}
	if parentID != "span2" {
		t.Errorf("parentID = %q, want %q (latest)", parentID, "span2")
	}
}

func TestCorrelator_ExpiresAfterTTL(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	c := sql.NewCorrelator(50 * time.Millisecond)
	c.SetActive("mytrace", "myspan")

	// Immediately should work
	traceID, _ := c.Correlate()
	if traceID != "mytrace" {
		t.Errorf("traceID = %q, want %q before expiry", traceID, "mytrace")
	}

	// After TTL, should get a new trace ID
	time.Sleep(100 * time.Millisecond)
	traceID, parentID := c.Correlate()
	if traceID == "mytrace" {
		t.Error("traceID should not be 'mytrace' after TTL expiry")
	}
	if len(traceID) != 32 {
		t.Errorf("traceID length = %d, want 32 (new trace ID)", len(traceID))
	}
	if parentID != "" {
		t.Errorf("parentID = %q, want empty after TTL expiry", parentID)
	}
}

func TestCorrelator_MultipleCorrelateCallsSameActive(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	c := sql.NewCorrelator(0)
	c.SetActive("mytrace", "myspan")

	for range 5 {
		traceID, parentID := c.Correlate()
		if traceID != "mytrace" {
			t.Errorf("traceID = %q, want %q", traceID, "mytrace")
		}
		if parentID != "myspan" {
			t.Errorf("parentID = %q, want %q", parentID, "myspan")
		}
	}
}
