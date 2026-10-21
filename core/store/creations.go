package store

import (
	"database/sql"

	"github.com/juncaifeng/digestly/core/model"
)

// ListCreations 列出全部创作单元(含素材源)。
func (s *Store) ListCreations() ([]model.Creation, error) {
	rows, err := s.db.Query(`SELECT id,name,group_name,template_id,created_at FROM creations ORDER BY group_name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Creation{}
	for rows.Next() {
		var c model.Creation
		if err := rows.Scan(&c.ID, &c.Name, &c.GroupName, &c.TemplateID, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	srcRows, err := s.db.Query(`SELECT creation_id,kind,ref_id FROM creation_sources ORDER BY kind, ref_id`)
	if err != nil {
		return nil, err
	}
	defer srcRows.Close()
	byCreation := map[int64][]model.CreationSource{}
	for srcRows.Next() {
		var cid int64
		var src model.CreationSource
		if err := srcRows.Scan(&cid, &src.Kind, &src.RefID); err != nil {
			return nil, err
		}
		byCreation[cid] = append(byCreation[cid], src)
	}
	if err := srcRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Sources = byCreation[out[i].ID]
	}
	return out, nil
}

// AddCreation 新建创作单元(含素材源)。
func (s *Store) AddCreation(c *model.Creation) error {
	res, err := s.db.Exec(`INSERT INTO creations(name,group_name,template_id) VALUES(?,?,?)`,
		c.Name, c.GroupName, c.TemplateID)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return s.replaceCreationSources(c.ID, c.Sources)
}

// UpdateCreation 更新名称/文件夹/模板,并整体替换素材源。
func (s *Store) UpdateCreation(c *model.Creation) error {
	res, err := s.db.Exec(`UPDATE creations SET name=?,group_name=?,template_id=? WHERE id=?`,
		c.Name, c.GroupName, c.TemplateID, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return s.replaceCreationSources(c.ID, c.Sources)
}

// DeleteCreation 删除创作单元(素材源级联,文章历史级联删除)。
func (s *Store) DeleteCreation(id int64) error {
	_, err := s.db.Exec(`DELETE FROM creations WHERE id=?`, id)
	return err
}

func (s *Store) replaceCreationSources(creationID int64, sources []model.CreationSource) error {
	if _, err := s.db.Exec(`DELETE FROM creation_sources WHERE creation_id=?`, creationID); err != nil {
		return err
	}
	for _, src := range sources {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO creation_sources(creation_id,kind,ref_id) VALUES(?,?,?)`,
			creationID, src.Kind, src.RefID); err != nil {
			return err
		}
	}
	return nil
}

// CreationFeedIDs 展开创作的素材源为订阅源 id 集合(专栏源展开为组内全部订阅)。
func (s *Store) CreationFeedIDs(creationID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT kind,ref_id FROM creation_sources WHERE creation_id=?`, creationID)
	if err != nil {
		return nil, err
	}
	kinds := []model.CreationSource{}
	for rows.Next() {
		var src model.CreationSource
		if err := rows.Scan(&src.Kind, &src.RefID); err != nil {
			rows.Close()
			return nil, err
		}
		kinds = append(kinds, src)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	var feedIDs []int64
	for _, src := range kinds {
		switch src.Kind {
		case "feed":
			if !seen[src.RefID] {
				seen[src.RefID] = true
				feedIDs = append(feedIDs, src.RefID)
			}
		case "column":
			fr, err := s.db.Query(`SELECT id FROM feeds WHERE column_id=?`, src.RefID)
			if err != nil {
				return nil, err
			}
			for fr.Next() {
				var fid int64
				if err := fr.Scan(&fid); err != nil {
					fr.Close()
					return nil, err
				}
				if !seen[fid] {
					seen[fid] = true
					feedIDs = append(feedIDs, fid)
				}
			}
			fr.Close()
		}
	}
	return feedIDs, nil
}
