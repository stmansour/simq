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

	// Optionally add additional checks to verify specific table structure elements

	// Drop the table after the test to avoid affecting subsequent tests
	err = qm.RemoveSchemaForTesting()
	if err != nil {
		t.Errorf("Failed to remove schema for testing: %v", err)
	}
}
