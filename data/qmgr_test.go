package data

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stmansour/simq/util"
)

// initTest initializes the test environment
func initTest(t *testing.T) (*QueueManager, error) {
	ex, err := util.ReadExternalResources()
	if err != nil {
		t.Errorf("Failed to read external resources: %v", err)
		return nil, err
	}
	cmd := ex.GetSQLOpenString("simq")

	// Initialize the queue manager
	qm, err := NewQueueManager(cmd)
	if err != nil {
		t.Errorf("Failed to initialize queue manager: %v", err)
		return nil, err
	}
	if err := qm.RemoveSchemaForTesting(); err != nil {
		return nil, err
	}
	if err := qm.EnsureSchemaExists(); err != nil {
		return nil, err
	}
	return qm, nil
}

// TestQueueManager tests the basic functionalities of QueueManager
func TestQueueManager(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}

	// Insert a new item
	newItem := QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       StateQueued,
		DtEstimate:  sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Errorf("Failed to insert item: %v", err)
	}
	fmt.Printf("Inserted item with SID %d\n", sid)

	// Update the item to set DtCompleted
	newItem.SID = int(sid)
	newItem.State = StateBooked
	newItem.DtCompleted = sql.NullTime{Time: time.Now(), Valid: true}
	err = qm.UpdateItem(newItem)
	if err != nil {
		t.Errorf("Failed to update item: %v", err)
	}
	fmt.Printf("Updated item with SID %d\n", newItem.SID)

	// Query queued and executing items
	items, err := qm.GetQueuedAndExecutingItems()
	if err != nil {
		t.Errorf("Failed to get items: %v", err)
	}
	fmt.Printf("Queued and executing items: %+v\n", items)

	// Handling NULL values when reading items
	for _, item := range items {
		if item.DtCompleted.Valid {
			fmt.Printf("Item %d completed at %s\n", item.SID, item.DtCompleted.Time)
		} else {
			fmt.Printf("Item %d has not completed yet\n", item.SID)
		}
	}

	// Delete the item
	err = qm.DeleteItem(int(sid))
	if err != nil {
		t.Errorf("Failed to delete item: %v", err)
	}
	fmt.Printf("Deleted item with SID %d\n", sid)
}

// TestUpdateItem_NullDtEstimate tests updating an item with NULL DtEstimate
func TestUpdateItem_NullDtEstimate(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}

	// Insert a new item
	newItem := QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       StateQueued,
		DtEstimate:  sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Errorf("Failed to insert item: %v", err)
		return
	}

	// Update the item with NULL DtEstimate
	newItem.SID = int(sid)
	newItem.DtEstimate = sql.NullTime{Valid: false}
	err = qm.UpdateItem(newItem)
	if err != nil {
		t.Errorf("Failed to update item: %v", err)
	}

	// Verify the DtEstimate is NULL in the database
	var updatedItem QueueItem
	err = qm.db.QueryRow(`SELECT DtEstimate FROM Queue WHERE SID = ?`, sid).Scan(&updatedItem.DtEstimate)
	if err != nil {
		t.Errorf("Failed to check updated item: %v", err)
		return
	}

	if updatedItem.DtEstimate.Valid {
		t.Errorf("Expected NULL DtEstimate in the database, but got a value")
	}

	// Drop the table after the test to avoid affecting subsequent tests
	err = qm.RemoveSchemaForTesting()
	if err != nil {
		t.Errorf("Failed to remove schema for testing: %v", err)
	}
}

// TestEnsureSchemaExists tests the schema creation process
func TestEnsureSchemaExists(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}

	// Drop the Queue table (if it exists) before testing
	err = qm.RemoveSchemaForTesting()
	if err != nil {
		t.Errorf("Failed to remove schema for testing: %v", err)
		return
	}

	// Ensure the schema exists (should create the table)
	err = qm.EnsureSchemaExists()
	if err != nil {
		t.Errorf("Failed to create schema: %v", err)
		return
	}

}

func TestGetActiveQueue(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}

	// Insert test items into the queue
	items := []QueueItem{
		{File: "file1", Name: "Simulation 1", Priority: 1, Description: "Test Simulation 1", URL: "http://localhost:8080", State: StateExecuting, DtEstimate: sql.NullTime{Time: time.Now().Add(10 * time.Hour), Valid: true}},
		{File: "file2", Name: "Simulation 2", Priority: 3, Description: "Test Simulation 2", URL: "http://localhost:8080", State: StateExecuting, DtEstimate: sql.NullTime{Time: time.Now().Add(8 * time.Hour), Valid: true}},
		{File: "file3", Name: "Simulation 3", Priority: 2, Description: "Test Simulation 3", URL: "http://localhost:8080", State: StateQueued},
		{File: "file4", Name: "Simulation 4", Priority: 5, Description: "Test Simulation 4", URL: "http://localhost:8080", State: StateBooked},
		{File: "file5", Name: "Simulation 5", Priority: 4, Description: "Test Simulation 5", URL: "http://localhost:8080", State: StateExecuting},
		{File: "file6", Name: "Simulation 6", Priority: 1, Description: "Test Simulation 6", URL: "http://localhost:8080", State: StateQueued},
		{File: "file7", Name: "Simulation 7", Priority: 2, Description: "Test Simulation 7", URL: "http://localhost:8080", State: StateBooked},
		{File: "file8", Name: "Simulation 8", Priority: 3, Description: "Test Simulation 8", URL: "http://localhost:8080", State: StateExecuting, DtEstimate: sql.NullTime{Time: time.Now().Add(12 * time.Hour), Valid: true}},
		{File: "file9", Name: "Simulation 9", Priority: 5, Description: "Test Simulation 9", URL: "http://localhost:8080", State: StateQueued},
		{File: "file10", Name: "Simulation 10", Priority: 4, Description: "Test Simulation 10", URL: "http://localhost:8080", State: StateBooked},
	}

	for _, item := range items {
		_, err := qm.InsertItem(item)
		if err != nil {
			t.Errorf("Failed to insert item: %v", err)
			return
		}
	}

	// Retrieve and verify the queue items
	queueItems, err := qm.GetQueuedAndExecutingItems()
	if err != nil {
		t.Errorf("Failed to get queue items: %v", err)
		return
	}

	expectedOrder := []int{2, 1, 8, 5, 6, 3, 7, 10, 4, 9}
	for i, item := range queueItems {
		if item.SID != expectedOrder[i] {
			t.Errorf("Item order mismatch at position %d: got %d want %d", i, item.SID, expectedOrder[i])
		}
	}
}

func TestGetItemByID(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}

	// Insert a new item
	newItem := QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       StateQueued,
		DtEstimate:  sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Errorf("Failed to insert item: %v", err)
		return
	}

	// Retrieve the item by SID
	retrievedItem, err := qm.GetItemByID(int(sid))
	if err != nil {
		t.Errorf("Failed to retrieve item by ID: %v", err)
		return
	}

	// Verify the retrieved item matches the inserted item
	if retrievedItem.SID != int(sid) {
		t.Errorf("Retrieved SID does not match: got %d want %d", retrievedItem.SID, int(sid))
	}
	if retrievedItem.File != newItem.File {
		t.Errorf("Retrieved File does not match: got %s want %s", retrievedItem.File, newItem.File)
	}
	if retrievedItem.Name != newItem.Name {
		t.Errorf("Retrieved Name does not match: got %s want %s", retrievedItem.Name, newItem.Name)
	}
	if retrievedItem.Priority != newItem.Priority {
		t.Errorf("Retrieved Priority does not match: got %d want %d", retrievedItem.Priority, newItem.Priority)
	}
	if retrievedItem.Description != newItem.Description {
		t.Errorf("Retrieved Description does not match: got %s want %s", retrievedItem.Description, newItem.Description)
	}
	if retrievedItem.URL != newItem.URL {
		t.Errorf("Retrieved URL does not match: got %s want %s", retrievedItem.URL, newItem.URL)
	}
	if retrievedItem.State != newItem.State {
		t.Errorf("Retrieved State does not match: got %d want %d", retrievedItem.State, newItem.State)
	}
	if !retrievedItem.DtEstimate.Valid || !newItem.DtEstimate.Valid || !timestampsClose(retrievedItem.DtEstimate.Time, newItem.DtEstimate.Time) {
		t.Errorf("Retrieved DtEstimate does not match: got %v want %v", retrievedItem.DtEstimate, newItem.DtEstimate)
	}
}

// timestampsClose checks if two timestamps are within 1 second of each other
func timestampsClose(t1, t2 time.Time) bool {
	return t1.Sub(t2) < time.Second && t2.Sub(t1) < time.Second
}

// TestDeleteItem tests the DeleteItem function
func TestDeleteItem(t *testing.T) {
	qm, err := initTest(t)
	if err != nil {
		return
	}

	// Insert a new item
	newItem := QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		State:       StateQueued,
		DtEstimate:  sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Errorf("Failed to insert item: %v", err)
		return
	}

	// Delete the item
	err = qm.DeleteItem(int(sid))
	if err != nil {
		t.Errorf("Failed to delete item: %v", err)
		return
	}

	// Try to retrieve the deleted item
	_, err = qm.GetItemByID(int(sid))
	if err == nil {
		t.Errorf("Expected error when retrieving deleted item, but got none")
	}
}
