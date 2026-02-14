package observability

import (
	"sync"
	"sync/atomic"
)

type Counter struct {
	value int64
}

func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

func (c *Counter) Add(n int64) {
	atomic.AddInt64(&c.value, n)
}

func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

type Gauge struct {
	value int64
}

func (g *Gauge) Set(v int64) {
	atomic.StoreInt64(&g.value, v)
}

func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}

type Histogram struct {
	mu     sync.Mutex
	values []float64
	sum    float64
	count  int64
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, v)
	h.sum += v
	h.count++
}

func (h *Histogram) Snapshot() (count int64, sum float64, avg float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return 0, 0, 0
	}
	return h.count, h.sum, h.sum / float64(h.count)
}

type MetricsRegistry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

func (r *MetricsRegistry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{}
	r.counters[name] = c
	return c
}

func (r *MetricsRegistry) Gauge(name string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{}
	r.gauges[name] = g
	return g
}

func (r *MetricsRegistry) Histogram(name string) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[name]; ok {
		return h
	}
	h := &Histogram{}
	r.histograms[name] = h
	return h
}

func (r *MetricsRegistry) Snapshot() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.RUnlock()

	result := make(map[string]interface{})

	for name, c := range r.counters {
		result["counter."+name] = c.Value()
	}

	for name, g := range r.gauges {
		result["gauge."+name] = g.Value()
	}
	for name, h := range r.histograms {
		count, sum, avg := h.Snapshot()
		result["histogram."+name+".count"] = count
		result["histogram."+name+".sum"] = sum
		result["histogram."+name+".avg"] = avg
	}
	return result
}
