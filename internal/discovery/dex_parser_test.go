package discovery

import (
	"testing"
)

func TestDEXParser_ParseSwapEvents_Empty(t *testing.T) {
	parser := NewDEXParser()

	events := parser.ParseSwapEvents(nil, "testsig", 100, 1700000000)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestDEXParser_RegisterParser(t *testing.T) {
	parser := NewDEXParser()

	// Default parsers should be registered
	if len(parser.parsers) != 2 {
		t.Errorf("expected 2 default parsers, got %d", len(parser.parsers))
	}

	if _, ok := parser.parsers[RaydiumAMMV4]; !ok {
		t.Error("Raydium parser not registered")
	}

	if _, ok := parser.parsers[PumpFun]; !ok {
		t.Error("PumpFun parser not registered")
	}
}

func TestPumpFunParser_ParseSwapEvents_Buy(t *testing.T) {
	parser := NewPumpFunParser()

	logs := []string{
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]",
		"Program log: mint=ABC123DEF456",
		"Program log: Instruction: Buy",
		"Program log: amount=1000000",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",
	}

	events := parser.ParseSwapEvents(logs, "testsig", 100, 1700000000)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Mint != "ABC123DEF456" {
		t.Errorf("expected mint ABC123DEF456, got %s", events[0].Mint)
	}

	if events[0].TxSignature != "testsig" {
		t.Errorf("expected signature testsig, got %s", events[0].TxSignature)
	}

	if events[0].Slot != 100 {
		t.Errorf("expected slot 100, got %d", events[0].Slot)
	}
}

func TestPumpFunParser_ParseSwapEvents_Sell(t *testing.T) {
	parser := NewPumpFunParser()

	logs := []string{
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]",
		"Program log: mint=XYZ789",
		"Program log: Instruction: Sell",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",
	}

	events := parser.ParseSwapEvents(logs, "sellsig", 200, 1700000001)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Mint != "XYZ789" {
		t.Errorf("expected mint XYZ789, got %s", events[0].Mint)
	}
}

func TestPumpFunParser_ParseSwapEvents_MultipleTrades(t *testing.T) {
	parser := NewPumpFunParser()

	logs := []string{
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]",
		"Program log: mint=TOKEN1",
		"Program log: Instruction: Buy",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]",
		"Program log: mint=TOKEN2",
		"Program log: Instruction: Sell",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",
	}

	events := parser.ParseSwapEvents(logs, "multisig", 300, 1700000002)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Mint != "TOKEN1" {
		t.Errorf("expected first mint TOKEN1, got %s", events[0].Mint)
	}

	if events[1].Mint != "TOKEN2" {
		t.Errorf("expected second mint TOKEN2, got %s", events[1].Mint)
	}

	// EventIndex should be ordered
	if events[0].EventIndex >= events[1].EventIndex {
		t.Errorf("expected events to be ordered by index")
	}
}

func TestPumpFunParser_ParseSwapEvents_NoPumpFun(t *testing.T) {
	parser := NewPumpFunParser()

	logs := []string{
		"Program 11111111111111111111111111111111 invoke [1]",
		"Program log: Transfer",
		"Program 11111111111111111111111111111111 success",
	}

	events := parser.ParseSwapEvents(logs, "testsig", 100, 1700000000)

	if len(events) != 0 {
		t.Errorf("expected 0 events for non-PumpFun logs, got %d", len(events))
	}
}

func TestGenericSwapParser_ParseSwapEvents(t *testing.T) {
	parser := NewGenericSwapParser()

	logs := []string{
		"Some random log",
		"Swap executed mint=ABC123DEF456GHI789JKL012MNO345PQR678",
		"Another log",
	}

	events := parser.ParseSwapEvents(logs, "testsig", 100, 1700000000)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Mint != "ABC123DEF456GHI789JKL012MNO345PQR678" {
		t.Errorf("unexpected mint: %s", events[0].Mint)
	}
}

func TestDEXParser_SortsByEventIndex(t *testing.T) {
	parser := NewDEXParser()

	// Create logs that would result in multiple events at different indices
	logs := []string{
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]",
		"Program log: mint=TOKEN1",
		"Program log: Instruction: Buy",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]",
		"Program log: mint=TOKEN2",
		"Program log: Instruction: Sell",
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",
	}

	events := parser.ParseSwapEvents(logs, "testsig", 100, 1700000000)

	// Verify events are sorted by EventIndex
	for i := 1; i < len(events); i++ {
		if events[i].EventIndex < events[i-1].EventIndex {
			t.Errorf("events not sorted: index %d (%d) < index %d (%d)",
				i, events[i].EventIndex, i-1, events[i-1].EventIndex)
		}
	}
}

func TestRaydiumParser_ParseSwapEvents_NoRayLog(t *testing.T) {
	parser := NewRaydiumParser()

	logs := []string{
		"Program 675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8 invoke [1]",
		"Program log: some log",
		"Program 675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8 success",
	}

	events := parser.ParseSwapEvents(logs, "testsig", 100, 1700000000)

	// Without ray_log, no events should be parsed
	if len(events) != 0 {
		t.Errorf("expected 0 events without ray_log, got %d", len(events))
	}
}

func TestRaydiumParser_ParseSwapEvents_InvalidBase64(t *testing.T) {
	parser := NewRaydiumParser()

	logs := []string{
		"ray_log: not-valid-base64!!!",
	}

	events := parser.ParseSwapEvents(logs, "testsig", 100, 1700000000)

	// Invalid base64 should be skipped
	if len(events) != 0 {
		t.Errorf("expected 0 events for invalid base64, got %d", len(events))
	}
}

func TestReadUint64LE(t *testing.T) {
	data := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	result := readUint64LE(data, 0)
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}

	// Test out of bounds
	result = readUint64LE(data, 5)
	if result != 0 {
		t.Errorf("expected 0 for out of bounds, got %d", result)
	}
}

func TestLiquidityEvent(t *testing.T) {
	// Test LiquidityEvent struct
	event := LiquidityEvent{
		Pool:        "pooladdr",
		Mint:        "mintaddr",
		EventType:   "add",
		AmountToken: 1000,
		AmountQuote: 2000,
		TxSignature: "sig",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1700000000,
	}

	if event.EventType != "add" {
		t.Errorf("unexpected event type: %s", event.EventType)
	}
}
