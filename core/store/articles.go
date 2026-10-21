package store

import (
	"database/sql"
	"encoding/json"

	"github.com/juncaifeng/digestly/core/model"
)

// --- 专栏文章模板 ---

// ListTemplates 列出全部生成模板(内置在前)。
func (s *Store) ListTemplates() ([]model.ArticleTemplate, error) {
	rows, err := s.db.Query(`SELECT id,name,prompt,built_in,created_at FROM article_templates ORDER BY built_in DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ArticleTemplate{}
	for rows.Next() {
		var t model.ArticleTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Prompt, &t.BuiltIn, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTemplate 取单个模板。
func (s *Store) GetTemplate(id int64) (*model.ArticleTemplate, error) {
	var t model.ArticleTemplate
	err := s.db.QueryRow(`SELECT id,name,prompt,built_in,created_at FROM article_templates WHERE id=?`, id).
		Scan(&t.ID, &t.Name, &t.Prompt, &t.BuiltIn, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// AddTemplate 新增自定义模板。
func (s *Store) AddTemplate(t *model.ArticleTemplate) error {
	res, err := s.db.Exec(`INSERT INTO article_templates(name,prompt) VALUES(?,?)`, t.Name, t.Prompt)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

// UpdateTemplate 更新模板名称与内容(内置模板同样可编辑)。
func (s *Store) UpdateTemplate(t *model.ArticleTemplate) error {
	res, err := s.db.Exec(`UPDATE article_templates SET name=?,prompt=? WHERE id=?`, t.Name, t.Prompt, t.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteTemplate 删除模板;内置模板禁止删除。
func (s *Store) DeleteTemplate(id int64) error {
	res, err := s.db.Exec(`DELETE FROM article_templates WHERE id=? AND built_in=0`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows // 不存在或为内置模板
	}
	return nil
}

// --- 专栏文章 ---

// AddArticle 保存生成的创作文章。
func (s *Store) AddArticle(a *model.Article) error {
	ids, err := json.Marshal(a.ItemIDs)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`INSERT INTO articles(creation_id,template_id,title,content,user_message,item_ids) VALUES(?,?,?,?,?,?)`,
		a.CreationID, a.TemplateID, a.Title, a.Content, a.UserMessage, string(ids))
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// ListArticles 列出一个创作单元的生成历史(新的在前,不含正文)。
func (s *Store) ListArticles(creationID int64) ([]model.Article, error) {
	rows, err := s.db.Query(`SELECT id,creation_id,template_id,title,user_message,item_ids,created_at,updated_at
		FROM articles WHERE creation_id=? ORDER BY id DESC`, creationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Article{}
	for rows.Next() {
		var a model.Article
		var ids string
		if err := rows.Scan(&a.ID, &a.CreationID, &a.TemplateID, &a.Title, &a.UserMessage, &ids, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ids), &a.ItemIDs)
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetArticle 取单篇(含正文)。
func (s *Store) GetArticle(id int64) (*model.Article, error) {
	var a model.Article
	var ids string
	err := s.db.QueryRow(`SELECT id,creation_id,template_id,title,content,user_message,item_ids,created_at,updated_at
		FROM articles WHERE id=?`, id).
		Scan(&a.ID, &a.CreationID, &a.TemplateID, &a.Title, &a.Content, &a.UserMessage, &ids, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(ids), &a.ItemIDs)
	return &a, nil
}

// UpdateArticle 编辑标题与正文。
func (s *Store) UpdateArticle(a *model.Article) error {
	res, err := s.db.Exec(`UPDATE articles SET title=?,content=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		a.Title, a.Content, a.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteArticle 删除一篇生成记录。
func (s *Store) DeleteArticle(id int64) error {
	_, err := s.db.Exec(`DELETE FROM articles WHERE id=?`, id)
	return err
}
