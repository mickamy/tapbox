package trace

type Store interface {
	Add(span *Span)
	GetTrace(traceID string) *Trace
	ListTraces(offset, limit int) []*Trace
	Subscribe() <-chan *Span
	Unsubscribe(ch <-chan *Span)
}
