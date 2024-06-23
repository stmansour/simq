package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/simq/util"
)

func TestQueueManager(t *testing.T) {
	ex := util.ReadExternalResources()
	// Initialize the queue manager
	qm, err := NewQueueManager("user:password@tcp(localhost:3306)/simq")
	if err != nil {
		t.Errorf("Failed to initialize queue manager: %v", err)
		return
	}

	// Insert a new item
	newItem := QueueItem{
		File:        "/path/to/simulation.tar.gz",
		Name:        "Test Simulation",
		Priority:    5,
		Description: "A test simulation",
		URL:         "http://localhost:8080",
		Status:      0,
		DtEstimate:  time.Now().Add(24 * time.Hour),
		DtCompleted: time.Time{},
	}
	sid, err := qm.InsertItem(newItem)
	if err != nil {
		t.Errorf("Failed to insert item: %v", err)
	}
	fmt.Printf("Inserted item with SID %d\n", sid)

	// Update the item
	newItem.SID = int(sid)
	newItem.Status = 2
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
}
