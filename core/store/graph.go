package store

import (
	"database/sql"
	"encoding/binary"
	"math"
	"strings"

	"github.com/juncaifeng/digestly/core/model"
)

// ---- 实体 / 关系 ----

// UpsertEntity 归一化后 upsert 实体,返回 id。
func (s *Store) UpsertEntity(name, typ string) (int64, error) {
	name = strings.TrimSpace(name)
	norm := strings.ToLower(name)
	var id int64
	err := s.db.QueryRow(`SELECT id FROM entities WHERE normalized=?`, norm).Scan(&id)
	if err == nil {
		if typ != "" {
			_, _ = s.db.Exec(`UPDATE entities SET type=? WHERE id=? AND type=''`, typ, id)
		}
		return id, nil
	}
	res, err := s.db.Exec(`INSERT INTO entities(name,type,normalized) VALUES(?,?,?)`, name, typ, norm)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LinkItemEntity 记录文章对实体的提及(重复提及累加 count)。
func (s *Store) LinkItemEntity(itemID, entityID int64, mention string, count int) error {
	if count <= 0 {
		count = 1
	}
	_, err := s.db.Exec(`INSERT INTO item_entities(item_id,entity_id,mention,count) VALUES(?,?,?,?)
		ON CONFLICT(item_id,entity_id) DO UPDATE SET mention=excluded.mention, count=count+excluded.count`,
		itemID, entityID, mention, count)
	return err
}

// AddRelation 写实体关系三元组(同 subject-predicate-object 幂等)。
func (s *Store) AddRelation(subjectID int64, predicate string, objectID int64, itemID int64, confidence float64) error {
	if confidence <= 0 {
		confidence = 1
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO relations(subject_id,predicate,object_id,item_id,confidence) VALUES(?,?,?,?,?)`,
		subjectID, strings.TrimSpace(predicate), objectID, itemID, confidence)
	return err
}

// Entity 实体(含提及统计)。
type Entity struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Normalized string `json:"normalized"`
	ItemCount  int    `json:"item_count"` // 提及该实体的文章数
}

// ListEntities 模糊列实体(按提及文章数降序)。
func (s *Store) ListEntities(query string, limit int) ([]Entity, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT e.id,e.name,e.type,e.normalized,
		(SELECT COUNT(*) FROM item_entities ie WHERE ie.entity_id=e.id)
		FROM entities e`
	args := []any{}
	if query = strings.TrimSpace(query); query != "" {
		q += ` WHERE e.name LIKE ? OR e.normalized LIKE ?`
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like)
	}
	q += ` ORDER BY 5 DESC, e.id LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entity{}
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Normalized, &e.ItemCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Relation 实体关系(带双方名称)。
type Relation struct {
	ID            int64   `json:"id"`
	Subject       string  `json:"subject"`
	Predicate     string  `json:"predicate"`
	Object        string  `json:"object"`
	SubjectID     int64   `json:"subject_id"`
	ObjectID      int64   `json:"object_id"`
	ItemID        int64   `json:"item_id"`
	Confidence    float64 `json:"confidence"`
}

// EntityRelations 取实体相关的全部关系(作为主语或宾语)。
func (s *Store) EntityRelations(entityID int64) ([]Relation, error) {
	rows, err := s.db.Query(`SELECT r.id,s.name,r.predicate,o.name,r.subject_id,r.object_id,r.item_id,r.confidence
		FROM relations r JOIN entities s ON s.id=r.subject_id JOIN entities o ON o.id=r.object_id
		WHERE r.subject_id=? OR r.object_id=? ORDER BY r.id`, entityID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Relation{}
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.ID, &r.Subject, &r.Predicate, &r.Object, &r.SubjectID, &r.ObjectID, &r.ItemID, &r.Confidence); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- 向量 ----

func vecToBlob(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func blobToVec(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// SetItemEmbedding 写文章向量(upsert)。
func (s *Store) SetItemEmbedding(itemID int64, model string, vec []float32) error {
	_, err := s.db.Exec(`INSERT INTO item_embeddings(item_id,model,dims,vec) VALUES(?,?,?,?)
		ON CONFLICT(item_id) DO UPDATE SET model=excluded.model,dims=excluded.dims,vec=excluded.vec,created_at=CURRENT_TIMESTAMP`,
		itemID, model, len(vec), vecToBlob(vec))
	return err
}

// SetEntityEmbedding 写实体向量(upsert)。
func (s *Store) SetEntityEmbedding(entityID int64, model string, vec []float32) error {
	_, err := s.db.Exec(`INSERT INTO entity_embeddings(entity_id,model,dims,vec) VALUES(?,?,?,?)
		ON CONFLICT(entity_id) DO UPDATE SET model=excluded.model,dims=excluded.dims,vec=excluded.vec,created_at=CURRENT_TIMESTAMP`,
		entityID, model, len(vec), vecToBlob(vec))
	return err
}

// SimilarItem 相似文章命中。
type SimilarItem struct {
	Item  model.Item `json:"item"`
	Score float32    `json:"score"`
}

// SimilarItems 按向量余弦找最相似文章(暴力扫描,十万级毫秒内)。
func (s *Store) SimilarItems(modelName string, vec []float32, limit int) ([]SimilarItem, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`SELECT item_id, vec FROM item_embeddings WHERE model=?`, modelName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type hit struct {
		id    int64
		score float32
	}
	var hits []hit
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		if id == 0 {
			continue
		}
		hits = append(hits, hit{id, Cosine(vec, blobToVec(blob))})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 简单选择前 N(按分数降序)
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].score > hits[i].score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]SimilarItem, 0, len(hits))
	for _, h := range hits {
		it, err := s.GetItem(h.id)
		if err != nil {
			continue // 向量残留但文章没了
		}
		out = append(out, SimilarItem{Item: *it, Score: h.score})
	}
	return out, nil
}

// Cosine 余弦相似度(任一为空返回 0)。
func Cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// GetItemEmbedding 取文章向量(模型名 + 向量);无向量时 ok=false。
func (s *Store) GetItemEmbedding(itemID int64) (string, []float32, bool, error) {
	var model string
	var blob []byte
	err := s.db.QueryRow(`SELECT model, vec FROM item_embeddings WHERE item_id=?`, itemID).Scan(&model, &blob)
	if err == sql.ErrNoRows {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, err
	}
	return model, blobToVec(blob), true, nil
}

// ItemEntities 取一篇文章提及的实体(名称/类型/提及次数,次数降序)。
func (s *Store) ItemEntities(itemID int64) ([]Entity, error) {
	rows, err := s.db.Query(`SELECT e.id,e.name,e.type,e.normalized,ie.count
		FROM item_entities ie JOIN entities e ON e.id=ie.entity_id
		WHERE ie.item_id=? ORDER BY ie.count DESC, e.id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entity{}
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Normalized, &e.ItemCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
