package store

import (
	"database/sql"
	"strings"

	"github.com/juncaifeng/digestly/core/model"
)

// maskAPIKey 掩码 api_key: 保留后 4 位,其余打码;短于等于 4 位全打码。
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

const llmConfigCols = `id,name,base_url,api_key,model,temperature,max_tokens,is_default,created_at,updated_at`

func scanLLMConfig(row interface{ Scan(...any) error }) (*model.LLMConfig, error) {
	var c model.LLMConfig
	if err := row.Scan(&c.ID, &c.Name, &c.BaseURL, &c.APIKey, &c.Model,
		&c.Temperature, &c.MaxTokens, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListLLMConfigs 列出全部 LLM 配置(api_key 掩码返回)。
func (s *Store) ListLLMConfigs() ([]model.LLMConfig, error) {
	rows, err := s.db.Query(`SELECT ` + llmConfigCols + ` FROM llm_configs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.LLMConfig{}
	for rows.Next() {
		c, err := scanLLMConfig(rows)
		if err != nil {
			return nil, err
		}
		c.APIKey = maskAPIKey(c.APIKey)
		out = append(out, *c)
	}
	return out, rows.Err()
}

// GetLLMConfig 取单个配置(含明文 api_key,仅服务端内部使用)。
func (s *Store) GetLLMConfig(id int64) (*model.LLMConfig, error) {
	return scanLLMConfig(s.db.QueryRow(`SELECT `+llmConfigCols+` FROM llm_configs WHERE id=?`, id))
}

// DefaultLLMConfig 取默认配置(含明文 api_key);无默认时返回 sql.ErrNoRows。
func (s *Store) DefaultLLMConfig() (*model.LLMConfig, error) {
	return scanLLMConfig(s.db.QueryRow(`SELECT `+llmConfigCols+` FROM llm_configs WHERE is_default=1 ORDER BY id LIMIT 1`))
}

// AddLLMConfig 新增配置;首个配置自动设为默认,显式 is_default 时清掉旧默认。
func (s *Store) AddLLMConfig(c *model.LLMConfig) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM llm_configs`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		c.IsDefault = true
	}
	if c.IsDefault {
		if _, err := s.db.Exec(`UPDATE llm_configs SET is_default=0`); err != nil {
			return err
		}
	}
	res, err := s.db.Exec(`INSERT INTO llm_configs(name,base_url,api_key,model,temperature,max_tokens,is_default)
		VALUES(?,?,?,?,?,?,?)`,
		c.Name, strings.TrimSpace(c.BaseURL), c.APIKey, c.Model, c.Temperature, c.MaxTokens, c.IsDefault)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

// UpdateLLMConfig 更新配置;APIKey 为空串表示不修改原密钥。
func (s *Store) UpdateLLMConfig(c *model.LLMConfig) error {
	if c.IsDefault {
		if _, err := s.db.Exec(`UPDATE llm_configs SET is_default=0 WHERE id!=?`, c.ID); err != nil {
			return err
		}
	}
	q := `UPDATE llm_configs SET name=?,base_url=?,model=?,temperature=?,max_tokens=?,is_default=?,updated_at=CURRENT_TIMESTAMP`
	args := []any{c.Name, strings.TrimSpace(c.BaseURL), c.Model, c.Temperature, c.MaxTokens, c.IsDefault}
	if c.APIKey != "" {
		q += `,api_key=?`
		args = append(args, c.APIKey)
	}
	q += ` WHERE id=?`
	args = append(args, c.ID)
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteLLMConfig 删除配置。
func (s *Store) DeleteLLMConfig(id int64) error {
	_, err := s.db.Exec(`DELETE FROM llm_configs WHERE id=?`, id)
	return err
}

// SetDefaultLLMConfig 指定默认配置(同时清掉其他默认)。
func (s *Store) SetDefaultLLMConfig(id int64) error {
	if _, err := s.db.Exec(`UPDATE llm_configs SET is_default=0`); err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE llm_configs SET is_default=1, updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetSetting 取应用设置(kv);无则返回 ok=false。
func (s *Store) GetSetting(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting 写应用设置(kv, upsert)。
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO app_settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
