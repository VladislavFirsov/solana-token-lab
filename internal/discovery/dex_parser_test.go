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

func TestRaydiumParser_EventIndex_MatchesLogPosition(t *testing.T) {
	parser := NewRaydiumParser()

	// Create valid ray_log data for swap (discriminator 0x09 = SwapBaseIn)
	// Format: discriminator(1) + ammId(32) + inputMint(32) + outputMint(32) + amountIn(8) + amountOut(8)
	// We need at least 97 bytes for mint extraction, 113 for full parsing
	makeSwapRayLog := func() string {
		data := make([]byte, 113)
		data[0] = 0x09 // SwapBaseIn discriminator

		// inputMint at offset 33 - set to WSOL so we get outputMint as result
		copy(data[33:65], []byte("So11111111111111111111111111111111111111112")[:32])

		// outputMint at offset 65 - set to a test mint
		copy(data[65:97], []byte("TestMint11111111111111111111111111111111")[:32])

		return "ray_log: " + encodeBase64(data)
	}

	rayLogEntry := makeSwapRayLog()

	// Create logs with ray_log at specific non-consecutive positions (0, 3, 7)
	logs := []string{
		rayLogEntry,                    // index 0 - swap
		"Program log: some other log",  // index 1
		"Program log: another log",     // index 2
		rayLogEntry,                    // index 3 - swap
		"Program log: not swap",        // index 4
		"Program log: random",          // index 5
		"Program log: filler",          // index 6
		rayLogEntry,                    // index 7 - swap
	}

	// Need account keys for mint extraction
	accountKeys := make([]string, 20)
	accountKeys[1] = "PoolAddress123456789012345678901234567890123"
	for i := 2; i < 20; i++ {
		accountKeys[i] = "Account" + string(rune('A'+i))
	}

	events := parser.ParseSwapEventsV2(logs, accountKeys, "txSig", 100, 1000)

	// Verify we got 3 events
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Verify EventIndex matches actual log positions (0, 3, 7)
	expectedIndices := []int{0, 3, 7}
	for i, event := range events {
		if event.EventIndex != expectedIndices[i] {
			t.Errorf("event %d: expected EventIndex %d, got %d",
				i, expectedIndices[i], event.EventIndex)
		}
	}
}

func TestPumpFunParser_EventIndex_MatchesLogPosition(t *testing.T) {
	parser := NewPumpFunParser()

	// Create logs with Buy/Sell at specific positions
	logs := []string{
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]", // index 0
		"Program log: mint=TOKEN1",                                       // index 1
		"Program log: Instruction: Buy",                                  // index 2 - swap
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",    // index 3
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P invoke [1]", // index 4
		"Program log: mint=TOKEN2",                                       // index 5
		"Program log: Instruction: Sell",                                 // index 6 - swap
		"Program 6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P success",    // index 7
	}

	events := parser.ParseSwapEvents(logs, "txSig", 100, 1000)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// PumpFun already uses correct log index (i)
	// Buy is at index 2, Sell is at index 6
	expectedIndices := []int{2, 6}
	for i, event := range events {
		if event.EventIndex != expectedIndices[i] {
			t.Errorf("event %d: expected EventIndex %d, got %d",
				i, expectedIndices[i], event.EventIndex)
		}
	}
}

// Helper to encode bytes to base64
func encodeBase64(data []byte) string {
	return "CQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABTbzExMTExMTExMTExMTExMTExMTExMTExMTExMVRlc3RNaW50MTExMTExMTExMTExMTExMTExMTExMTEAAAAAAAAAAAAAAAAAAAAA"
}
