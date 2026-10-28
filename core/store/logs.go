package store

// PipelineLog 管线单步骤执行记录。
type PipelineLog struct {
	ID         int64  `json:"id"`
	ItemID     int64  `json:"item_id"`
	Step       string `json:"step"`
	Status     string `json:"status"` // ok | failed | skipped
	DurationMs int64  `json:"duration_ms"`
	Message    string `json:"message"`
	CreatedAt  string `json:"created_at"`
}

// AddPipelineLog 写一条管线执行日志。
func (s *Store) AddPipelineLog(itemID int64, step, status string, durationMs int64, message string) error {
	_, err := s.db.Exec(`INSERT INTO pipeline_logs(item_id,step,status,duration_ms,message) VALUES(?,?,?,?,?)`,
		itemID, step, status, durationMs, message)
	return err
}

// PipelineLogs 取一篇文章的管线执行日志(新→旧,含历次重跑)。
func (s *Store) PipelineLogs(itemID int64) ([]PipelineLog, error) {
	rows, err := s.db.Query(`SELECT id,item_id,step,status,duration_ms,message,created_at
		FROM pipeline_logs WHERE item_id=? ORDER BY id DESC`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PipelineLog{}
	for rows.Next() {
		var l PipelineLog
		if err := rows.Scan(&l.ID, &l.ItemID, &l.Step, &l.Status, &l.DurationMs, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
