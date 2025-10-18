package models

import "sync/atomic"

type Clicks struct {
	Count uint64 `json:"count"`
}

func (c *Clicks) Increment() {
	atomic.AddUint64(&c.Count, 1)
}

func (c *Clicks) GetCount() uint64 {
	return atomic.LoadUint64(&c.Count)
}

var ClicksStore = &Clicks{Count: 0}
