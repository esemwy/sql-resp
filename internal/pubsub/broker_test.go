package pubsub

import (
	"sync"
	"testing"
)

type mockSub struct {
	mu       sync.Mutex
	received []string
}

func (m *mockSub) Deliver(channel, message string) {
	m.mu.Lock()
	m.received = append(m.received, channel+":"+message)
	m.mu.Unlock()
}

func (m *mockSub) Messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.received))
	copy(cp, m.received)
	return cp
}

func TestSubscribePublish(t *testing.T) {
	b := New()
	sub := &mockSub{}

	b.Subscribe("news", sub)
	b.Publish("news", "hello")

	msgs := sub.Messages()
	if len(msgs) != 1 || msgs[0] != "news:hello" {
		t.Fatalf("expected [news:hello], got %v", msgs)
	}
}

func TestNoSubscribers(t *testing.T) {
	b := New()
	n := b.Publish("empty", "msg")
	if n != 0 {
		t.Errorf("expected 0 subscribers, got %d", n)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := New()
	s1 := &mockSub{}
	s2 := &mockSub{}

	b.Subscribe("ch", s1)
	b.Subscribe("ch", s2)

	n := b.Publish("ch", "hi")
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
	if len(s1.Messages()) != 1 || len(s2.Messages()) != 1 {
		t.Error("both subscribers should have received the message")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := New()
	sub := &mockSub{}

	b.Subscribe("ch", sub)
	b.Unsubscribe("ch", sub)
	b.Publish("ch", "late")

	if len(sub.Messages()) != 0 {
		t.Error("should not receive after unsubscribe")
	}
}

func TestUnsubscribeAll(t *testing.T) {
	b := New()
	sub := &mockSub{}

	b.Subscribe("ch1", sub)
	b.Subscribe("ch2", sub)
	b.Unsubscribe("", sub)

	b.Publish("ch1", "m1")
	b.Publish("ch2", "m2")

	if len(sub.Messages()) != 0 {
		t.Error("should not receive after unsubscribe all")
	}
}

func TestSubscribeCount(t *testing.T) {
	b := New()
	sub := &mockSub{}

	n := b.Subscribe("ch1", sub)
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
	n = b.Subscribe("ch2", sub)
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}
