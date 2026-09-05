package task

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{name: "pending to queued", from: StatusPending, to: StatusQueued, want: true},
		{name: "pending to failed", from: StatusPending, to: StatusFailed, want: true},
		{name: "queued to running", from: StatusQueued, to: StatusRunning, want: true},
		{name: "queued to failed", from: StatusQueued, to: StatusFailed, want: true},
		{name: "running to succeeded", from: StatusRunning, to: StatusSucceeded, want: true},
		{name: "running to failed", from: StatusRunning, to: StatusFailed, want: true},
		{name: "pending cannot skip to running", from: StatusPending, to: StatusRunning, want: false},
		{name: "running cannot return to queued", from: StatusRunning, to: StatusQueued, want: false},
		{name: "succeeded is terminal", from: StatusSucceeded, to: StatusRunning, want: false},
		{name: "failed is terminal", from: StatusFailed, to: StatusQueued, want: false},
		{name: "unknown source", from: Status("unknown"), to: StatusQueued, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanTransition(test.from, test.to); got != test.want {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestStatusIsTerminal(t *testing.T) {
	if !StatusSucceeded.IsTerminal() {
		t.Fatal("succeeded must be terminal")
	}
	if !StatusFailed.IsTerminal() {
		t.Fatal("failed must be terminal")
	}
	if StatusRunning.IsTerminal() {
		t.Fatal("running must not be terminal")
	}
}
