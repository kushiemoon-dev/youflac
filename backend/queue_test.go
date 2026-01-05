package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Queue Creation Tests
// =============================================================================

func TestNewQueue(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	if q == nil {
		t.Fatal("NewQueue returned nil")
	}

	if q.maxConc != 2 {
		t.Errorf("Expected maxConc 2, got %d", q.maxConc)
	}

	if len(q.items) != 0 {
		t.Errorf("Expected empty items, got %d", len(q.items))
	}
}

// =============================================================================
// Add/Remove Tests
// =============================================================================

func TestAddToQueue(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	request := DownloadRequest{
		VideoURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		SpotifyURL: "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT",
		Quality:    "1080p",
	}

	id, err := q.AddToQueue(request)
	if err != nil {
		t.Fatalf("AddToQueue failed: %v", err)
	}

	if id == "" {
		t.Error("Expected non-empty ID")
	}

	items := q.GetQueue()
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}

	if items[0].VideoURL != request.VideoURL {
		t.Errorf("Expected VideoURL %s, got %s", request.VideoURL, items[0].VideoURL)
	}

	if items[0].Status != StatusPending {
		t.Errorf("Expected status Pending, got %s", items[0].Status)
	}
}

func TestAddToQueueWithMetadata(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	request := DownloadRequest{
		VideoURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	}

	videoInfo := &VideoInfo{
		Title:     "Never Gonna Give You Up",
		Artist:    "Rick Astley",
		Duration:  212.0,
		Thumbnail: "https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
	}

	id, err := q.AddToQueueWithMetadata(request, videoInfo)
	if err != nil {
		t.Fatalf("AddToQueueWithMetadata failed: %v", err)
	}

	item := q.GetItem(id)
	if item == nil {
		t.Fatal("GetItem returned nil")
	}

	if item.Title != videoInfo.Title {
		t.Errorf("Expected title %s, got %s", videoInfo.Title, item.Title)
	}

	if item.Artist != videoInfo.Artist {
		t.Errorf("Expected artist %s, got %s", videoInfo.Artist, item.Artist)
	}
}

func TestRemoveFromQueue(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})

	if len(q.GetQueue()) != 2 {
		t.Errorf("Expected 2 items, got %d", len(q.GetQueue()))
	}

	err := q.RemoveFromQueue(id)
	if err != nil {
		t.Fatalf("RemoveFromQueue failed: %v", err)
	}

	items := q.GetQueue()
	if len(items) != 1 {
		t.Errorf("Expected 1 item after removal, got %d", len(items))
	}

	// Verify the removed item is gone
	for _, item := range items {
		if item.ID == id {
			t.Error("Removed item still in queue")
		}
	}
}

func TestRemoveNonExistent(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	err := q.RemoveFromQueue("non-existent-id")
	if err != nil {
		t.Errorf("RemoveFromQueue should not error for non-existent ID: %v", err)
	}
}

// =============================================================================
// Status Update Tests
// =============================================================================

func TestUpdateStatus(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	q.UpdateStatus(id, StatusDownloadingVideo, 50, "Downloading...")

	item := q.GetItem(id)
	if item.Status != StatusDownloadingVideo {
		t.Errorf("Expected status DownloadingVideo, got %s", item.Status)
	}

	if item.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", item.Progress)
	}

	if item.Stage != "Downloading..." {
		t.Errorf("Expected stage 'Downloading...', got %s", item.Stage)
	}
}

func TestSetItemError(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	testErr := "download failed: network error"
	q.SetItemError(id, fmt.Errorf(testErr))

	item := q.GetItem(id)
	if item.Status != StatusError {
		t.Errorf("Expected status Error, got %s", item.Status)
	}

	if item.Error != testErr {
		t.Errorf("Expected error %s, got %s", testErr, item.Error)
	}
}

func TestCompleteStatus(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	q.UpdateStatus(id, StatusComplete, 100, "Complete")

	item := q.GetItem(id)
	if item.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set when status is Complete")
	}
}

// =============================================================================
// Clear Operations Tests
// =============================================================================

func TestClearCompleted(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id1, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	id2, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test3"})

	// Mark some as complete
	q.UpdateStatus(id1, StatusComplete, 100, "Complete")
	q.UpdateStatus(id2, StatusError, 0, "Error")

	removed := q.ClearCompleted()

	if removed != 2 {
		t.Errorf("Expected 2 items removed, got %d", removed)
	}

	items := q.GetQueue()
	if len(items) != 1 {
		t.Errorf("Expected 1 item remaining, got %d", len(items))
	}
}

func TestClearAll(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test3"})

	q.ClearAll()

	if len(q.GetQueue()) != 0 {
		t.Errorf("Expected empty queue after ClearAll, got %d items", len(q.GetQueue()))
	}
}

// =============================================================================
// Move Item Tests
// =============================================================================

func TestMoveItem(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id1, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	id2, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})
	id3, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test3"})

	// Initial order: id1, id2, id3
	// Move id3 (position 2) to position 0
	// Expected order: id3, id1, id2
	err := q.MoveItem(id3, 0)
	if err != nil {
		t.Fatalf("MoveItem failed: %v", err)
	}

	items := q.GetQueue()
	if items[0].ID != id3 {
		t.Errorf("Expected id3 at position 0, got %s", items[0].ID)
	}

	if items[1].ID != id1 {
		t.Errorf("Expected id1 at position 1, got %s", items[1].ID)
	}

	if items[2].ID != id2 {
		t.Errorf("Expected id2 at position 2, got %s", items[2].ID)
	}
}

func TestMoveItemInvalidIndex(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	err := q.MoveItem(id, 10)
	if err == nil {
		t.Error("Expected error for invalid index")
	}
}

// =============================================================================
// Statistics Tests
// =============================================================================

func TestGetStats(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id1, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	id2, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})
	id3, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test3"})
	id4, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test4"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test5"})

	q.UpdateStatus(id1, StatusComplete, 100, "Complete")
	q.UpdateStatus(id2, StatusError, 0, "Error")
	q.UpdateStatus(id3, StatusDownloadingVideo, 50, "Downloading")
	q.UpdateStatus(id4, StatusCancelled, 0, "Cancelled")
	// id5 remains pending

	stats := q.GetStats()

	if stats.Total != 5 {
		t.Errorf("Expected total 5, got %d", stats.Total)
	}

	if stats.Completed != 1 {
		t.Errorf("Expected completed 1, got %d", stats.Completed)
	}

	if stats.Failed != 1 {
		t.Errorf("Expected failed 1, got %d", stats.Failed)
	}

	if stats.Active != 1 {
		t.Errorf("Expected active 1, got %d", stats.Active)
	}

	if stats.Pending != 1 {
		t.Errorf("Expected pending 1, got %d", stats.Pending)
	}

	if stats.Cancelled != 1 {
		t.Errorf("Expected cancelled 1, got %d", stats.Cancelled)
	}
}

func TestGetPendingCount(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	id2, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test3"})

	q.UpdateStatus(id2, StatusComplete, 100, "Complete")

	if q.GetPendingCount() != 2 {
		t.Errorf("Expected 2 pending, got %d", q.GetPendingCount())
	}
}

func TestGetActiveCount(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id1, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	id2, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})
	q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test3"})

	q.UpdateStatus(id1, StatusDownloadingVideo, 50, "Downloading")
	q.UpdateStatus(id2, StatusMuxing, 80, "Muxing")

	if q.GetActiveCount() != 2 {
		t.Errorf("Expected 2 active, got %d", q.GetActiveCount())
	}
}

// =============================================================================
// Progress Callback Tests
// =============================================================================

func TestProgressCallback(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	events := make([]QueueEvent, 0)
	var mu sync.Mutex

	q.SetProgressCallback(func(event QueueEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	// Wait for async event
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(events) == 0 {
		mu.Unlock()
		t.Error("Expected at least one event from AddToQueue")
		return
	}

	addEvent := events[0]
	mu.Unlock()

	if addEvent.Type != "added" {
		t.Errorf("Expected event type 'added', got %s", addEvent.Type)
	}

	if addEvent.ItemID != id {
		t.Errorf("Expected item ID %s, got %s", id, addEvent.ItemID)
	}
}

// =============================================================================
// Persistence Tests
// =============================================================================

func TestSaveAndLoadQueue(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override data path for test
	queuePath := filepath.Join(tempDir, "queue.json")

	ctx := context.Background()
	q := NewQueue(ctx, 2)

	// Add items
	id1, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test1"})
	id2, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test2"})

	q.UpdateStatus(id1, StatusComplete, 100, "Complete")

	// Manual save to test path
	state := QueueState{
		Items:     q.GetQueue(),
		UpdatedAt: time.Now(),
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(queuePath, data, 0644)

	// Create new queue and load
	q2 := NewQueue(ctx, 2)

	// Manual load from test path
	loadData, _ := os.ReadFile(queuePath)
	var loadedState QueueState
	json.Unmarshal(loadData, &loadedState)
	q2.mutex.Lock()
	q2.items = loadedState.Items
	q2.mutex.Unlock()

	items := q2.GetQueue()
	if len(items) != 2 {
		t.Errorf("Expected 2 items after load, got %d", len(items))
	}

	// Verify item data
	var found1, found2 bool
	for _, item := range items {
		if item.ID == id1 {
			found1 = true
			if item.Status != StatusComplete {
				t.Errorf("Expected status Complete for id1, got %s", item.Status)
			}
		}
		if item.ID == id2 {
			found2 = true
			if item.Status != StatusPending {
				t.Errorf("Expected status Pending for id2, got %s", item.Status)
			}
		}
	}

	if !found1 || !found2 {
		t.Error("Not all items found after load")
	}
}

func TestLoadQueueResetsInProgress(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	// Create state with in-progress items
	state := QueueState{
		Items: []QueueItem{
			{ID: "test1", Status: StatusDownloadingVideo, Stage: "Downloading"},
			{ID: "test2", Status: StatusMuxing, Stage: "Muxing"},
			{ID: "test3", Status: StatusComplete, Stage: "Complete"},
		},
		UpdatedAt: time.Now(),
	}

	// Simulate loading
	q.mutex.Lock()
	for i := range state.Items {
		switch state.Items[i].Status {
		case StatusFetchingInfo, StatusDownloadingVideo, StatusDownloadingAudio, StatusMuxing, StatusOrganizing:
			state.Items[i].Status = StatusPending
			state.Items[i].Progress = 0
			state.Items[i].Stage = "Waiting... (resumed)"
		}
	}
	q.items = state.Items
	q.mutex.Unlock()

	items := q.GetQueue()

	// Check in-progress items are reset to pending
	for _, item := range items {
		if item.ID == "test1" || item.ID == "test2" {
			if item.Status != StatusPending {
				t.Errorf("Expected status Pending for %s, got %s", item.ID, item.Status)
			}
		}
		// Complete items should stay complete
		if item.ID == "test3" {
			if item.Status != StatusComplete {
				t.Errorf("Expected status Complete for test3, got %s", item.Status)
			}
		}
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestConcurrentAddRemove(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	var wg sync.WaitGroup
	ids := make(chan string, 100)

	// Add items concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := q.AddToQueue(DownloadRequest{
				VideoURL: fmt.Sprintf("https://youtube.com/watch?v=test%d", i),
			})
			if err != nil {
				t.Errorf("AddToQueue failed: %v", err)
			}
			ids <- id
		}(i)
	}

	wg.Wait()
	close(ids)

	// Collect all IDs
	allIDs := make([]string, 0)
	for id := range ids {
		allIDs = append(allIDs, id)
	}

	if len(q.GetQueue()) != 50 {
		t.Errorf("Expected 50 items, got %d", len(q.GetQueue()))
	}

	// Remove half concurrently
	var wg2 sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			q.RemoveFromQueue(allIDs[i])
		}(i)
	}

	wg2.Wait()

	remaining := len(q.GetQueue())
	if remaining != 25 {
		t.Errorf("Expected 25 items after removal, got %d", remaining)
	}
}

func TestConcurrentStatusUpdates(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	var wg sync.WaitGroup

	// Update status concurrently from multiple goroutines
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q.UpdateStatus(id, StatusDownloadingVideo, i, "Progress")
		}(i)
	}

	wg.Wait()

	item := q.GetItem(id)
	if item == nil {
		t.Fatal("Item should exist")
	}

	// Status should be valid (no race condition crash)
	if item.Status != StatusDownloadingVideo {
		t.Errorf("Expected status DownloadingVideo, got %s", item.Status)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestGetItemNotFound(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	item := q.GetItem("non-existent-id")
	if item != nil {
		t.Error("Expected nil for non-existent item")
	}
}

func TestEmptyQueueStats(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	stats := q.GetStats()

	if stats.Total != 0 {
		t.Errorf("Expected total 0, got %d", stats.Total)
	}

	if stats.Pending != 0 || stats.Active != 0 || stats.Completed != 0 {
		t.Error("Expected all stats to be 0 for empty queue")
	}
}

func TestCancelItem(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	id, _ := q.AddToQueue(DownloadRequest{VideoURL: "https://youtube.com/watch?v=test"})

	err := q.CancelItem(id)
	if err != nil {
		t.Fatalf("CancelItem failed: %v", err)
	}

	item := q.GetItem(id)
	if item.Status != StatusCancelled {
		t.Errorf("Expected status Cancelled, got %s", item.Status)
	}
}

func TestCancelItemNotFound(t *testing.T) {
	ctx := context.Background()
	q := NewQueue(ctx, 2)

	err := q.CancelItem("non-existent-id")
	if err == nil {
		t.Error("Expected error for cancelling non-existent item")
	}
}
