package enforcement

import (
	"context"
	"testing"
	"time"
)

func TestProcessIdentifier(t *testing.T) {
	identifier := NewProcessIdentifier()

	// Add a signature
	signature := &ProcessSignature{
		Name:         "test-app",
		Path:         "/usr/bin/test-app",
		MatchMethods: []MatchMethod{MatchByName, MatchByPath},
	}

	identifier.AddSignature(signature)

	// Test identification by name
	process := &ProcessInfo{
		Name: "test-app",
		Path: "/usr/bin/test-app",
	}

	matched, found := identifier.IdentifyProcess(process)
	if !found {
		t.Error("Expected to find matching signature")
	}
	if matched.Name != signature.Name {
		t.Errorf("Expected signature name %s, got %s", signature.Name, matched.Name)
	}

	// Test non-matching process
	unknownProcess := &ProcessInfo{
		Name: "unknown-app",
		Path: "/usr/bin/unknown-app",
	}

	_, found = identifier.IdentifyProcess(unknownProcess)
	if found {
		t.Error("Expected not to find matching signature for unknown process")
	}
}

func TestLinuxProcessMonitor(t *testing.T) {
	ctx := context.Background()
	monitor := NewLinuxProcessMonitor(100 * time.Millisecond)

	// Test getting processes
	processes, err := monitor.GetProcesses(ctx)
	if err != nil {
		t.Fatalf("Failed to get processes: %v", err)
	}

	if len(processes) == 0 {
		t.Error("Expected at least some processes to be running")
	}

	// Verify process info structure
	for _, proc := range processes {
		if proc.PID <= 0 {
			t.Errorf("Invalid PID: %d", proc.PID)
		}
		// Name might be empty for some system processes, that's ok
	}
}

func TestLinuxProcessMonitorStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	monitor := NewLinuxProcessMonitor(50 * time.Millisecond)

	// Test starting
	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	// Test that it's running
	if !monitor.isRunning() {
		t.Error("Monitor should be running after start")
	}

	// Test subscribing to events
	eventCh := monitor.Subscribe()

	// Wait a bit for events
	select {
	case event := <-eventCh:
		t.Logf("Received event: %s for process %s (PID: %d)",
			event.Type, event.Process.Name, event.Process.PID)
	case <-time.After(500 * time.Millisecond):
		// No events is also acceptable for a short test
		t.Log("No process events received in short test period")
	}

	// Test stopping
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop monitor: %v", err)
	}

	if monitor.isRunning() {
		t.Error("Monitor should not be running after stop")
	}
}

func TestProcessEventTypes(t *testing.T) {
	// Test event type constants
	if ProcessStarted != "started" {
		t.Errorf("Expected ProcessStarted to be 'started', got %s", ProcessStarted)
	}

	if ProcessStopped != "stopped" {
		t.Errorf("Expected ProcessStopped to be 'stopped', got %s", ProcessStopped)
	}
}

func TestMatchMethods(t *testing.T) {
	// Test match method constants
	expectedMethods := map[MatchMethod]string{
		MatchByPath: "path",
		MatchByHash: "hash",
		MatchByName: "name",
	}

	for method, expected := range expectedMethods {
		if string(method) != expected {
			t.Errorf("Expected match method %s to be %s, got %s",
				method, expected, string(method))
		}
	}
}

func TestProcessInfoValidation(t *testing.T) {
	process := &ProcessInfo{
		PID:         1234,
		PPID:        1,
		Name:        "test-process",
		Path:        "/usr/bin/test",
		CommandLine: "test --flag",
		StartTime:   time.Now(),
	}

	// Validate required fields
	if process.PID <= 0 {
		t.Error("PID should be positive")
	}

	if process.Name == "" {
		t.Error("Name should not be empty")
	}

	if process.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
}

// Benchmark process enumeration performance
func BenchmarkLinuxProcessMonitorGetProcesses(b *testing.B) {
	ctx := context.Background()
	monitor := NewLinuxProcessMonitor(1 * time.Second)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := monitor.GetProcesses(ctx)
		if err != nil {
			b.Fatalf("Failed to get processes: %v", err)
		}
	}
}

// Test process monitoring under load
func TestProcessMonitorConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	monitor := NewLinuxProcessMonitor(10 * time.Millisecond)

	err := monitor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Create multiple subscribers
	const numSubscribers = 5
	eventChannels := make([]<-chan ProcessEvent, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		eventChannels[i] = monitor.Subscribe()
	}

	// Collect events from all subscribers for a short time
	done := make(chan bool)
	eventCounts := make([]int, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		go func(idx int) {
			for {
				select {
				case <-eventChannels[idx]:
					eventCounts[idx]++
				case <-done:
					return
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	// Let it run for a bit
	time.Sleep(200 * time.Millisecond)
	close(done)

	// All subscribers should receive similar number of events
	// (This is a rough test - exact counts may vary)
	t.Logf("Event counts per subscriber: %v", eventCounts)
}
