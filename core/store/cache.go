package store

import (
	"container/list"
	"sort"
	"sync"

	"github.com/juncaifeng/digestly/core/model"
)

// --- L1: 条目元数据内存索引(常驻,不含正文) ---
// 列表/过滤/排序/分页全部走内存,零 SQL;写操作先落库再同步索引(write-through)。

type itemMeta struct {
	item    model.Item // Content 恒为空
	tags    []string
	favCols []int64 // 手动收藏进的专栏 id
}

type itemIndex struct {
	mu       sync.RWMutex
	items    map[int64]*itemMeta
	order    []int64         // 与 SQL 一致: published_at DESC(NULL 最后), id DESC
	feedCols map[int64]int64 // feed_id -> column_id(订阅继承通道)
}

func newItemIndex() *itemIndex {
	return &itemIndex{items: map[int64]*itemMeta{}, feedCols: map[int64]int64{}}
}

// ranksBefore 排序比较: 与 ORDER BY published_at DESC, id DESC 等价
// (SQLite 中 NULL 视为最小,DESC 时排最后)。
func ranksBefore(a, b *itemMeta) bool {
	ap, bp := a.item.PublishedAt, b.item.PublishedAt
	if ap == nil && bp != nil {
		return false
	}
	if ap != nil && bp == nil {
		return true
	}
	if ap != nil && bp != nil && !ap.Equal(*bp) {
		return ap.After(*bp)
	}
	return a.item.ID > b.item.ID
}

func (ix *itemIndex) add(m *itemMeta) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.items[m.item.ID] = m
	pos := sort.Search(len(ix.order), func(i int) bool {
		return ranksBefore(m, ix.items[ix.order[i]])
	})
	ix.order = append(ix.order, 0)
	copy(ix.order[pos+1:], ix.order[pos:])
	ix.order[pos] = m.item.ID
}

func (ix *itemIndex) remove(id int64) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if _, ok := ix.items[id]; !ok {
		return
	}
	delete(ix.items, id)
	for i, oid := range ix.order {
		if oid == id {
			ix.order = append(ix.order[:i], ix.order[i+1:]...)
			break
		}
	}
}

func (ix *itemIndex) update(id int64, fn func(m *itemMeta)) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if m, ok := ix.items[id]; ok {
		fn(m)
	}
}

// filter 按条件在有序序列上筛选并分页,返回副本(不含正文)。
func (ix *itemIndex) filter(f ItemFilter) []model.Item {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	out := []model.Item{}
	skipped := 0
	for _, id := range ix.order {
		m := ix.items[id]
		it := m.item
		if f.FeedID > 0 && it.FeedID != f.FeedID {
			continue
		}
		if f.Status != "" && it.Status != f.Status {
			continue
		}
		if f.Unread && it.Read {
			continue
		}
		if it.Archived != f.Archived {
			continue
		}
		if f.Tag != "" && !hasTag(m.tags, f.Tag) {
			continue
		}
		// 专栏过滤: 订阅继承 ∪ 手动收藏
		if f.ColumnID > 0 && ix.feedCols[it.FeedID] != f.ColumnID && !hasCol(m.favCols, f.ColumnID) {
			continue
		}
		if skipped < f.Offset {
			skipped++
			continue
		}
		cp := it
		cp.Tags = append([]string{}, m.tags...)
		out = append(out, cp)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func hasCol(cols []int64, id int64) bool {
	for _, c := range cols {
		if c == id {
			return true
		}
	}
	return false
}

func hasTag(tags []string, name string) bool {
	for _, t := range tags {
		if t == name {
			return true
		}
	}
	return false
}

// --- L2: 正文 LRU 缓存池(按字节限额) ---

type contentEntry struct {
	id      int64
	content string
	size    int64
}

type contentCache struct {
	mu       sync.Mutex
	maxBytes int64
	bytes    int64
	ll       *list.List // front = 最近使用
	elems    map[int64]*list.Element
}

func newContentCache(maxBytes int64) *contentCache {
	return &contentCache{maxBytes: maxBytes, ll: list.New(), elems: map[int64]*list.Element{}}
}

func (c *contentCache) Get(id int64) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.elems[id]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(*contentEntry).content, true
	}
	return "", false
}

func (c *contentCache) Add(id int64, content string) {
	size := int64(len(content))
	if size > c.maxBytes { // 单条超限不缓存
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.elems[id]; ok {
		c.bytes -= e.Value.(*contentEntry).size
		e.Value = &contentEntry{id, content, size}
		c.bytes += size
		c.ll.MoveToFront(e)
		return
	}
	c.elems[id] = c.ll.PushFront(&contentEntry{id, content, size})
	c.bytes += size
	for c.bytes > c.maxBytes && c.ll.Len() > 0 {
		back := c.ll.Back()
		ent := back.Value.(*contentEntry)
		c.bytes -= ent.size
		delete(c.elems, ent.id)
		c.ll.Remove(back)
	}
}

func (c *contentCache) Remove(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.elems[id]; ok {
		c.bytes -= e.Value.(*contentEntry).size
		delete(c.elems, id)
		c.ll.Remove(e)
	}
}

// Stats 缓存观测指标。
func (c *contentCache) Stats() (entries int, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len(), c.bytes
}
