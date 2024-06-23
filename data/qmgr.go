package data

import (
	"database/sql"
	"time"
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
	Status      int
	DtEstimate  time.Time
	DtCompleted time.Time
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
	err = manager.ensureTableExists()
	if err != nil {
		return nil, err
	}

	return manager, nil
}

// ensureTableExists creates the Queue table if it does not exist
func (qm *QueueManager) ensureTableExists() error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS Queue (
		SID INT AUTO_INCREMENT PRIMARY KEY,
		File VARCHAR(80) NOT NULL,
		Name VARCHAR(80) NOT NULL DEFAULT '',
		Priority INT NOT NULL DEFAULT 5,
		Description VARCHAR(256) NOT NULL DEFAULT '',
		URL VARCHAR(80) NOT NULL,
		Status INT NOT NULL DEFAULT 0,
		DtEstimate DATETIME,
		DtCompleted DATETIME,
		Created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		Modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	);`
	_, err := qm.db.Exec(createTableSQL)
	return err
}

// InsertItem inserts an item into the queue
func (qm *QueueManager) InsertItem(item QueueItem) (int64, error) {
	insertSQL := `INSERT INTO Queue (File, Name, Priority, Description, URL, Status, DtEstimate, DtCompleted)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := qm.db.Exec(insertSQL, item.File, item.Name, item.Priority, item.Description, item.URL, item.Status, item.DtEstimate, item.DtCompleted)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateItem updates an item in the queue
func (qm *QueueManager) UpdateItem(item QueueItem) error {
	updateSQL := `UPDATE Queue SET File = ?, Name = ?, Priority = ?, Description = ?, URL = ?, Status = ?, DtEstimate = ?, DtCompleted = ?, Modified = CURRENT_TIMESTAMP
				  WHERE SID = ?`
	_, err := qm.db.Exec(updateSQL, item.File, item.Name, item.Priority, item.Description, item.URL, item.Status, item.DtEstimate, item.DtCompleted, item.SID)
	return err
}

// GetQueuedAndExecutingItems returns all items in the queue
func (qm *QueueManager) GetQueuedAndExecutingItems() ([]QueueItem, error) {
	querySQL := `SELECT SID, File, Name, Priority, Description, URL, Status, DtEstimate, DtCompleted, Created, Modified
				 FROM Queue WHERE Status = 0 OR Status = 2`
	rows, err := qm.db.Query(querySQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var item QueueItem
		err := rows.Scan(&item.SID, &item.File, &item.Name, &item.Priority, &item.Description, &item.URL, &item.Status, &item.DtEstimate, &item.DtCompleted, &item.Created, &item.Modified)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}
