package api

import (
	"testing"
)

func TestWSClientSubscribe(t *testing.T) {
	client := &WSClient{
		subscriptions: make(map[EventType]bool),
	}

	events := []EventType{EventTypeAlert, EventTypeOrderUpdate}
	client.Subscribe(events)

	if !client.IsSubscribed(EventTypeAlert) {
		t.Error("Expected Alert to be subscribed")
	}
	if !client.IsSubscribed(EventTypeOrderUpdate) {
		t.Error("Expected OrderUpdate to be subscribed")
	}
}

func TestWSClientUnsubscribe(t *testing.T) {
	client := &WSClient{
		subscriptions: make(map[EventType]bool),
	}

	// Subscribe first
	events := []EventType{EventTypeAlert, EventTypeOrderUpdate}
	client.Subscribe(events)

	// Then unsubscribe from one
	client.Unsubscribe([]EventType{EventTypeAlert})

	if client.IsSubscribed(EventTypeAlert) {
		t.Error("Expected Alert to be unsubscribed")
	}
	if !client.IsSubscribed(EventTypeOrderUpdate) {
		t.Error("Expected OrderUpdate to still be subscribed")
	}
}

func TestWSClientDefaultAllSubscribed(t *testing.T) {
	client := &WSClient{
		subscriptions: make(map[EventType]bool),
	}

	// With no subscriptions, client should receive all events
	if !client.IsSubscribed(EventTypeAlert) {
		t.Error("Expected all events to be subscribed by default")
	}
	if !client.IsSubscribed(EventTypeStatus) {
		t.Error("Expected all events to be subscribed by default")
	}
}

func TestWSClientGetSubscriptions(t *testing.T) {
	client := &WSClient{
		subscriptions: make(map[EventType]bool),
	}

	events := []EventType{EventTypeAlert, EventTypeOrderUpdate}
	client.Subscribe(events)

	subs := client.GetSubscriptions()
	if len(subs) != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", len(subs))
	}
}

func TestWSClientHasSubscriptions(t *testing.T) {
	client := &WSClient{
		subscriptions: make(map[EventType]bool),
	}

	if client.HasSubscriptions() {
		t.Error("Expected no subscriptions initially")
	}

	client.Subscribe([]EventType{EventTypeAlert})
	if !client.HasSubscriptions() {
		t.Error("Expected subscriptions after Subscribe")
	}
}