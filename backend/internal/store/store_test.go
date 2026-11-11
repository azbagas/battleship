package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStore_SchemaAndCRUD(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_battleship.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("failed to close store: %v", err)
		}
	}()

	ctx := context.Background()

	slot1 := 1
	slot2 := 2

	now := time.Now().Truncate(time.Second)

	testCases := []struct {
		name       string
		record     *GameRecord
		wantWinner *int
	}{
		{
			name: "all_sunk normal victory host",
			record: &GameRecord{
				ID:          "game-all-sunk-001",
				StartedAt:   now.Add(-10 * time.Minute),
				EndedAt:     now,
				Player1Name: "Admiral Nimitz",
				Player2Name: "Admiral Yamamoto",
				WinnerSlot:  &slot1,
				EndReason:   "all_sunk",
				TotalShots:  34,
				ShotLog:     `[{"shooterId":"p1","cell":{"x":0,"y":0},"result":"hit","auto":false}]`,
			},
			wantWinner: &slot1,
		},
		{
			name: "forfeit guest win",
			record: &GameRecord{
				ID:          "game-forfeit-002",
				StartedAt:   now.Add(-5 * time.Minute),
				EndedAt:     now,
				Player1Name: "Host Quitter",
				Player2Name: "Guest Victor",
				WinnerSlot:  &slot2,
				EndReason:   "forfeit",
				TotalShots:  12,
				ShotLog:     `[{"shooterId":"p1","cell":{"x":1,"y":1},"result":"miss","auto":false}]`,
			},
			wantWinner: &slot2,
		},
		{
			name: "disconnect_timeout host win",
			record: &GameRecord{
				ID:          "game-timeout-003",
				StartedAt:   now.Add(-8 * time.Minute),
				EndedAt:     now,
				Player1Name: "Connected Host",
				Player2Name: "Dropped Guest",
				WinnerSlot:  &slot1,
				EndReason:   "disconnect_timeout",
				TotalShots:  20,
				ShotLog:     `[{"shooterId":"p2","cell":{"x":4,"y":4},"result":"hit","auto":true}]`,
			},
			wantWinner: &slot1,
		},
		{
			name: "abandoned both disconnected",
			record: &GameRecord{
				ID:          "game-abandoned-004",
				StartedAt:   now.Add(-15 * time.Minute),
				EndedAt:     now,
				Player1Name: "Ghost 1",
				Player2Name: "Ghost 2",
				WinnerSlot:  nil,
				EndReason:   "abandoned",
				TotalShots:  6,
				ShotLog:     `[]`,
			},
			wantWinner: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.SaveGame(ctx, tc.record); err != nil {
				t.Fatalf("SaveGame failed: %v", err)
			}

			retrieved, err := store.GetGame(ctx, tc.record.ID)
			if err != nil {
				t.Fatalf("GetGame failed: %v", err)
			}

			if retrieved.ID != tc.record.ID {
				t.Errorf("expected ID %q, got %q", tc.record.ID, retrieved.ID)
			}
			if retrieved.Player1Name != tc.record.Player1Name {
				t.Errorf("expected Player1Name %q, got %q", tc.record.Player1Name, retrieved.Player1Name)
			}
			if retrieved.Player2Name != tc.record.Player2Name {
				t.Errorf("expected Player2Name %q, got %q", tc.record.Player2Name, retrieved.Player2Name)
			}
			if retrieved.EndReason != tc.record.EndReason {
				t.Errorf("expected EndReason %q, got %q", tc.record.EndReason, retrieved.EndReason)
			}
			if retrieved.TotalShots != tc.record.TotalShots {
				t.Errorf("expected TotalShots %d, got %d", tc.record.TotalShots, retrieved.TotalShots)
			}
			if retrieved.ShotLog != tc.record.ShotLog {
				t.Errorf("expected ShotLog %q, got %q", tc.record.ShotLog, retrieved.ShotLog)
			}

			if tc.wantWinner == nil {
				if retrieved.WinnerSlot != nil {
					t.Errorf("expected WinnerSlot nil, got %v", *retrieved.WinnerSlot)
				}
			} else {
				if retrieved.WinnerSlot == nil {
					t.Fatalf("expected WinnerSlot %d, got nil", *tc.wantWinner)
				}
				if *retrieved.WinnerSlot != *tc.wantWinner {
					t.Errorf("expected WinnerSlot %d, got %d", *tc.wantWinner, *retrieved.WinnerSlot)
				}
			}
		})
	}

	// Test non-existent game lookup
	_, err = store.GetGame(ctx, "non-existent-id")
	if err == nil {
		t.Errorf("expected error when getting non-existent game, got nil")
	}

	// Test duplicate ID insertion rejected
	dupRecord := testCases[0].record
	if err := store.SaveGame(ctx, dupRecord); err == nil {
		t.Errorf("expected error on duplicate primary key insertion, got nil")
	}
}

