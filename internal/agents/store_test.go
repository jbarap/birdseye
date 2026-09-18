package agents

import (
	"encoding/json"
	"testing"
	"time"
)

// TestRecordClaimWireFormat pins the one place the in-memory claim and the on-disk record meet.
// In memory the tasks and the time a turn end asserted them are one value, so neither half can be
// built without the other; on disk they stay two flat sibling keys, `background` and
// `background_at`. That keeps every record an older build wrote readable and the file itself easy
// to read by eye, which is how these records actually get inspected.
func TestRecordClaimWireFormat(t *testing.T) {
	at := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tasks := []BackgroundTask{{Type: "monitor"}, {Type: "subagent", AgentType: "Explore"}}

	data, err := json.Marshal(record{SessionID: "s", Background: &claim{At: at, Tasks: tasks}})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	var wireTasks []BackgroundTask
	if err := json.Unmarshal(wire["background"], &wireTasks); err != nil {
		t.Fatalf("tasks should stay a bare `background` array: %v", err)
	}
	if len(wireTasks) != 2 || wireTasks[1].AgentType != "Explore" {
		t.Errorf("background = %+v, want both tasks in order", wireTasks)
	}
	var wireAt time.Time
	if err := json.Unmarshal(wire["background_at"], &wireAt); err != nil {
		t.Fatalf("background_at should be a timestamp of its own: %v", err)
	}
	if !wireAt.Equal(at) {
		t.Errorf("background_at = %v, want %v", wireAt, at)
	}

	// Round trip: what was written reads back as the same single claim.
	var back record
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Background.tasks()) != 2 || !back.Background.At.Equal(at) {
		t.Errorf("round trip = %+v, want the two tasks asserted at %v", back.Background, at)
	}

	// Nothing in flight carries neither key, so "no claim" cannot be confused with a claim that
	// asserted an empty set.
	data, err = json.Marshal(record{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	wire = map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["background"]; ok {
		t.Error("a record with no work in flight should carry no background key")
	}
	if _, ok := wire["background_at"]; ok {
		t.Error("a record with no work in flight should carry no background_at key")
	}

	// A record written before the timestamp existed: tasks with no background_at read back as a
	// claim of unknown age, which is treated as infinitely old and so expires on an idle row.
	var undated record
	if err := json.Unmarshal([]byte(`{"session_id":"s","background":[{"type":"shell"}]}`), &undated); err != nil {
		t.Fatal(err)
	}
	if len(undated.Background.tasks()) != 1 || !undated.Background.At.IsZero() {
		t.Errorf("undated record = %+v, want one task with a zero timestamp", undated.Background)
	}
	if got := effectiveTasks(undated.Background, StatusIdle, at); got != nil {
		t.Errorf("an undated claim should expire on an idle row, got %+v", got)
	}
	if got := effectiveTasks(undated.Background, StatusWorking, at); len(got) != 1 {
		t.Errorf("a working session should keep even an undated claim, got %+v", got)
	}
}

// TestRecordRoundTripThroughDisk pins that a record survives the real write/read path with its
// claim intact - the marshalling above is only correct if writeRecord and readRecord agree on it.
func TestRecordRoundTripThroughDisk(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	in := record{
		SessionID: "s1", PID: 42, TmuxPane: "%1", Title: "t", Status: StatusWorking,
		Background: &claim{At: at, Tasks: []BackgroundTask{{Type: "monitor"}}},
		Updated:    at,
	}
	if err := writeRecord(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := readRecord(dir, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Background.tasks()) != 1 || out.Background.Tasks[0].Type != "monitor" {
		t.Errorf("tasks did not survive the disk round trip, got %+v", out.Background)
	}
	if !out.Background.At.Equal(at) {
		t.Errorf("claim timestamp = %v, want %v", out.Background.At, at)
	}
	if out.Status != StatusWorking || out.PID != 42 {
		t.Errorf("the rest of the record must survive too, got %+v", out)
	}
}
