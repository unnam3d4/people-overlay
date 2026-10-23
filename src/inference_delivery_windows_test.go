//go:build windows

package main

import (
	"testing"
	"time"
)

// The old 500ms reply timeout could drop a result while a capture stalled.
func TestInferenceReplySurvivesSlowConsumer(t *testing.T) {
	replies := make(chan inferenceReply, 1)
	replies <- inferenceReply{epoch: 1}
	finished := make(chan bool, 1)
	go func() { finished <- deliverInferenceReply(replies, inferenceReply{epoch: 2}, nil) }()
	select {
	case <-finished:
		t.Fatal("reply disappeared while the buffer was full")
	case <-time.After(650 * time.Millisecond):
	}
	if first := <-replies; first.epoch != 1 {
		t.Fatalf("first reply overwritten: %d", first.epoch)
	}
	select {
	case ok := <-finished:
		if !ok { t.Fatal("reply cancelled without shutdown") }
	case <-time.After(2 * time.Second):
		t.Fatal("blocked reply did not resume")
	}
	if second := <-replies; second.epoch != 2 {
		t.Fatalf("second reply missing: %d", second.epoch)
	}
}

func TestInferenceReplyCancellationUnblocksShutdown(t *testing.T) {
	replies := make(chan inferenceReply, 1)
	replies <- inferenceReply{epoch: 1}
	quit := make(chan struct{})
	finished := make(chan bool, 1)
	go func() { finished <- deliverInferenceReply(replies, inferenceReply{epoch: 2}, quit) }()
	close(quit)
	select {
	case sent := <-finished:
		if sent { t.Fatal("full reply queue accepted cancelled response") }
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown cannot unblock worker")
	}
	if first := <-replies; first.epoch != 1 {
		t.Fatal("cancellation mutated an already queued result")
	}
}
