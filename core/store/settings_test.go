package store

import (
	"testing"

	"github.com/juncaifeng/digestly/core/model"
)

func TestLLMConfigCRUDAndDefault(t *testing.T) {
	st := openTest(t)

	// 首个配置自动成为默认
	c1 := &model.LLMConfig{Name: "a", BaseURL: "http://a/v1", APIKey: "sk-aaaa1111", Model: "m1"}
	if err := st.AddLLMConfig(c1); err != nil {
		t.Fatal(err)
	}
	if !c1.IsDefault {
		t.Fatal("first config should become default")
	}

	c2 := &model.LLMConfig{Name: "b", BaseURL: "http://b/v1", APIKey: "sk-bbbb2222", Model: "m2", IsDefault: true}
	if err := st.AddLLMConfig(c2); err != nil {
		t.Fatal(err)
	}

	def, err := st.DefaultLLMConfig()
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != c2.ID || def.APIKey != "sk-bbbb2222" {
		t.Fatalf("default = %+v, want c2 with plaintext key", def)
	}

	// 列表接口掩码
	list, _ := st.ListLLMConfigs()
	for _, c := range list {
		if c.APIKey != "" && len(c.APIKey) > 4 && c.APIKey[:4] != "****" {
			t.Fatalf("api_key not masked: %s", c.APIKey)
		}
	}

	// 空 api_key 更新不改密钥
	c2.Name = "b2"
	c2.APIKey = ""
	if err := st.UpdateLLMConfig(c2); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetLLMConfig(c2.ID)
	if got.Name != "b2" || got.APIKey != "sk-bbbb2222" {
		t.Fatalf("after update: name=%s key=%s", got.Name, got.APIKey)
	}

	// SetDefault 唯一性
	if err := st.SetDefaultLLMConfig(c1.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = st.ListLLMConfigs()
	n := 0
	for _, c := range list {
		if c.IsDefault {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("default count = %d, want 1", n)
	}
}

func TestBuiltinTemplatesSeeded(t *testing.T) {
	st := openTest(t)
	tpls, err := st.ListTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(tpls) < 2 {
		t.Fatalf("builtin templates = %d, want >= 2", len(tpls))
	}
	for _, tp := range tpls[:2] {
		if !tp.BuiltIn {
			t.Fatalf("template %s should be built_in", tp.Name)
		}
	}
	// 内置模板不可删除
	if err := st.DeleteTemplate(tpls[0].ID); err == nil {
		t.Fatal("builtin template should not be deletable")
	}
	// 自定义模板可增删
	ct := &model.ArticleTemplate{Name: "custom", Prompt: "p {{items}}"}
	if err := st.AddTemplate(ct); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTemplate(ct.ID); err != nil {
		t.Fatal(err)
	}
}

func TestArticleCRUD(t *testing.T) {
	st := openTest(t)
	cr := &model.Creation{Name: "art-creation"}
	if err := st.AddCreation(cr); err != nil {
		t.Fatal(err)
	}

	a := &model.Article{CreationID: cr.ID, TemplateID: 1, Title: "t", Content: "c", ItemIDs: []int64{1, 2, 3}}
	if err := st.AddArticle(a); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetArticle(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "t" || got.CreationID != cr.ID || len(got.ItemIDs) != 3 {
		t.Fatalf("got %+v", got)
	}
	list, _ := st.ListArticles(cr.ID)
	if len(list) != 1 {
		t.Fatalf("list = %d, want 1", len(list))
	}
	got.Title = "t2"
	if err := st.UpdateArticle(got); err != nil {
		t.Fatal(err)
	}
	got2, _ := st.GetArticle(a.ID)
	if got2.Title != "t2" {
		t.Fatalf("title = %s, want t2", got2.Title)
	}
	if err := st.DeleteArticle(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetArticle(a.ID); err == nil {
		t.Fatal("should be deleted")
	}
}

func TestCreationSourcesAndFeedIDs(t *testing.T) {
	st := openTest(t)
	f1 := addFeed(t, st, "f1")
	f2 := addFeed(t, st, "f2")
	f3 := addFeed(t, st, "f3")
	cid := mustColumn(t, st, "col")
	if err := st.SetFeedColumn(f1, &cid); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFeedColumn(f2, &cid); err != nil {
		t.Fatal(err)
	}

	cr := &model.Creation{
		Name: "c1",
		Sources: []model.CreationSource{
			{Kind: "column", RefID: cid},
			{Kind: "feed", RefID: f3},
			{Kind: "feed", RefID: f1}, // 与专栏展开重复,应去重
		},
	}
	if err := st.AddCreation(cr); err != nil {
		t.Fatal(err)
	}
	got, _ := st.ListCreations()
	if len(got) != 1 || len(got[0].Sources) != 3 {
		t.Fatalf("sources = %+v", got)
	}
	fids, err := st.CreationFeedIDs(cr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fids) != 3 {
		t.Fatalf("feedIDs = %v, want 3 unique", fids)
	}

	// 更新整体替换素材源
	cr.Sources = []model.CreationSource{{Kind: "feed", RefID: f2}}
	if err := st.UpdateCreation(cr); err != nil {
		t.Fatal(err)
	}
	fids, _ = st.CreationFeedIDs(cr.ID)
	if len(fids) != 1 || fids[0] != f2 {
		t.Fatalf("after update feedIDs = %v, want [f2]", fids)
	}
}

func mustColumn(t *testing.T, st *Store, name string) int64 {
	t.Helper()
	id, err := st.AddColumn(name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
