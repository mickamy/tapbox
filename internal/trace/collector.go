package trace

// Collector receives completed spans and submits them to the store.
type Collector struct {
	store Store
}

func NewCollector(store Store) *Collector {
	return &Collector{store: store}
}

func (c *Collector) Submit(span *Span) {
	c.store.Add(span)
}
