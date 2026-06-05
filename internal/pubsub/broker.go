// Package pubsub implements an in-process pub/sub message broker.
// No SQL is involved — all state is in-memory.
package pubsub

import (
	"sync"
)

// Subscriber receives messages on a channel.
type Subscriber interface {
	// Deliver sends a message to the subscriber. It must not block indefinitely.
	Deliver(channel, message string)
}

// Broker fans out PUBLISH messages to all subscribed connections.
type Broker struct {
	mu   sync.RWMutex
	subs map[string][]Subscriber // channel → subscribers
}

// New creates a new Broker.
func New() *Broker {
	return &Broker{subs: map[string][]Subscriber{}}
}

// Subscribe registers sub on channel. Returns the new subscription count for sub.
func (b *Broker) Subscribe(channel string, sub Subscriber) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[channel] = append(b.subs[channel], sub)
	return b.countForSub(sub)
}

// Unsubscribe removes sub from channel. Returns the remaining subscription count.
// If channel is "", removes sub from all channels.
func (b *Broker) Unsubscribe(channel string, sub Subscriber) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if channel == "" {
		for ch := range b.subs {
			b.subs[ch] = removeOne(b.subs[ch], sub)
			if len(b.subs[ch]) == 0 {
				delete(b.subs, ch)
			}
		}
		return 0
	}
	b.subs[channel] = removeOne(b.subs[channel], sub)
	if len(b.subs[channel]) == 0 {
		delete(b.subs, channel)
	}
	return b.countForSub(sub)
}

// Publish delivers message to all subscribers of channel. Returns subscriber count.
func (b *Broker) Publish(channel, message string) int {
	b.mu.RLock()
	subs := make([]Subscriber, len(b.subs[channel]))
	copy(subs, b.subs[channel])
	b.mu.RUnlock()

	for _, sub := range subs {
		sub.Deliver(channel, message)
	}
	return len(subs)
}

// countForSub returns the number of channels sub is subscribed to (called with lock held).
func (b *Broker) countForSub(sub Subscriber) int {
	count := 0
	for _, subs := range b.subs {
		for _, s := range subs {
			if s == sub {
				count++
			}
		}
	}
	return count
}

func removeOne(subs []Subscriber, target Subscriber) []Subscriber {
	result := subs[:0]
	removed := false
	for _, s := range subs {
		if !removed && s == target {
			removed = true
			continue
		}
		result = append(result, s)
	}
	return result
}
