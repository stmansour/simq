package data

import (
	"database/sql"
	"time"

	// Register the SQL driver
	_ "github.com/go-sql-driver/mysql"
)

const (
	// StateQueued indicates that the item is queued
	StateQueued = 0
	// StateBooked indicates that the item is booked for execution
	StateBooked = 1
	// StateExecuting indicates that the  item is currently executing
	StateExecuting = 2
	// StateCompleted indicates that the item has completed execution
	StateCompleted = 3
	// StateError indicates that there was an error with the item
	StateError = 4
)

// QueueManager is a wrapper around the MySQL database
type QueueManager struct {
	db *sql.DB
}

// QueueItem is an item in the queue
type QueueItem struct {
	SID         int
	File        string
	Name        string
	Priority    int
	Description string
	URL         string
	State       int
	DtEstimate  sql.NullTime
	DtCompleted sql.NullTime
	Created     time.Time
	Modified    time.Time
}

// NewQueueManager creates a new QueueManager
func NewQueueManager(dataSourceName string) (*QueueManager, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, err
	}

	manager := &QueueManager{db: db}

	return manager, nil
}

func (qm *QueueManager) executeCmdList(cmds []string) error {
	for _, cmd := range cmds {
		_, err := qm.db.Exec(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveSchemaForTesting removes the Queue table
func (qm *QueueManager) RemoveSchemaForTesting() error {
	stmts := []string{
		"DROP TABLE IF EXISTS Queue;",
	}
	return qm.executeCmdList(stmts)

}

// EnsureSchemaExists creates the Queue table if it does not exist
func (qm *QueueManager) EnsureSchemaExists() error {
	cmds := []string{
		`CREATE TABLE IF NOT EXISTS Queue (
		SID INT AUTO_INCREMENT PRIMARY KEY,
		File VARCHAR(80) NOT NULL,
		Name VARCHAR(80) NOT NULL DEFAULT '',
		Priority INT NOT NULL DEFAULT 5,
		Description VARCHAR(256) NOT NULL DEFAULT '',
		URL VARCHAR(80) NOT NULL,
		State INT NOT NULL DEFAULT 0,
		DtEstimate DATETIME,
		DtCompleted DATETIME,
		Created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		Modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	);`,
	}
	return qm.executeCmdList(cmds)
}

// InsertItem inserts an item into the queue
func (qm *QueueManager) InsertItem(item QueueItem) (int64, error) {
	insertSQL := `INSERT INTO Queue (File, Name, Priority, Description, URL, State, DtEstimate)
				  VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := qm.db.Exec(insertSQL, item.File, item.Name, item.Priority, item.Description, item.URL, item.State, item.DtEstimate)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateItem updates an item in the queue
func (qm *QueueManager) UpdateItem(item QueueItem) error {
	updateSQL := `UPDATE Queue SET File = ?, Name = ?, Priority = ?, Description = ?, URL = ?, State = ?, DtEstimate = ?, DtCompleted = ?, Modified = CURRENT_TIMESTAMP
				  WHERE SID = ?`
	_, err := qm.db.Exec(updateSQL, item.File, item.Name, item.Priority, item.Description, item.URL, item.State, item.DtEstimate, item.DtCompleted, item.SID)
	return err
}

// DeleteItem deletes an item from the queue
func (qm *QueueManager) DeleteItem(SID int) error {
	deleteSQL := `DELETE FROM Queue WHERE SID = ?`
	_, err := qm.db.Exec(deleteSQL, SID)
	return err
}

// GetQueuedAndExecutingItems returns all items in the queue
func (qm *QueueManager) GetQueuedAndExecutingItems() ([]QueueItem, error) {
	querySQL := `SELECT SID, File, Name, Priority, Description, URL, State, DtEstimate, DtCompleted, Created, Modified
				 FROM Queue WHERE (State >= 0 AND State <= 2) OR (State = 4)`
	rows, err := qm.db.Query(querySQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var item QueueItem
		err := rows.Scan(&item.SID, &item.File, &item.Name, &item.Priority, &item.Description, &item.URL, &item.State, &item.DtEstimate, &item.DtCompleted, &item.Created, &item.Modified)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}
