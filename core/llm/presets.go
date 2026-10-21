package llm

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ProviderPreset 一个 LLM 供应商预设(schema 见 providers/deepseek.yml 头注释)。
type ProviderPreset struct {
	Name    string         `json:"name" yaml:"name"`
	BaseURL string         `json:"base_url" yaml:"base_url"`
	Docs    string         `json:"docs" yaml:"docs"`
	Note    string         `json:"note" yaml:"note"`
	Models  []ModelPreset  `json:"models" yaml:"models"`
}

// ModelPreset 供应商的一个可选模型。
type ModelPreset struct {
	ID          string `json:"id" yaml:"id"`
	Label       string `json:"label" yaml:"label"`
	Context     string `json:"context" yaml:"context"`
	MaxOutput   string `json:"max_output" yaml:"max_output"`
	Thinking    string `json:"thinking" yaml:"thinking"`
	Vision      bool   `json:"vision" yaml:"vision"`
	Pricing     string `json:"pricing" yaml:"pricing"`
	Concurrency int    `json:"concurrency" yaml:"concurrency"`
}

//go:embed providers agents
var builtinFS embed.FS

// Presets 返回全部供应商预设: 内置(embed) + 用户目录(userDir 可空)。
// 用户目录里的同名文件覆盖内置预设,便于自行更新维护。
func Presets(userDir string) ([]ProviderPreset, error) {
	byName := map[string]ProviderPreset{}
	if err := loadDirFS(byName, builtinFS, "providers"); err != nil {
		return nil, err
	}
	if userDir != "" {
		entries, err := os.ReadDir(userDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || (filepath.Ext(e.Name()) != ".yml" && filepath.Ext(e.Name()) != ".yaml") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(userDir, e.Name()))
			if err != nil {
				return nil, err
			}
			p, err := parse(raw)
			if err != nil {
				return nil, err
			}
			byName[p.Name] = p
		}
	}
	out := make([]ProviderPreset, 0, len(byName))
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func loadDirFS(m map[string]ProviderPreset, fsys embed.FS, dir string) error {
	return fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".yml" && filepath.Ext(path) != ".yaml" {
			return nil
		}
		raw, err := fsys.ReadFile(path)
		if err != nil {
			return err
		}
		p, err := parse(raw)
		if err != nil {
			return err
		}
		m[p.Name] = p
		return nil
	})
}

func parse(raw []byte) (ProviderPreset, error) {
	var p ProviderPreset
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	return p, nil
}

// AgentPreset 外部 Agent 运行时预设(schema 见 agents/dsh.yml 头注释)。
type AgentPreset struct {
	Name    string `json:"name" yaml:"name"`
	Command string `json:"command" yaml:"command"`
	Docs    string `json:"docs" yaml:"docs"`
	Note    string `json:"note" yaml:"note"`
}

// AgentPresets 返回全部外部 Agent 运行时预设: 内置(embed) + 用户目录(userDir 可空)。
// 用户目录里的同名文件覆盖内置预设,便于自行更新维护。
func AgentPresets(userDir string) ([]AgentPreset, error) {
	byName := map[string]AgentPreset{}
	if err := fs.WalkDir(builtinFS, "agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".yml" && filepath.Ext(path) != ".yaml" {
			return nil
		}
		raw, err := builtinFS.ReadFile(path)
		if err != nil {
			return err
		}
		var p AgentPreset
		if err := yaml.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.Name != "" {
			byName[p.Name] = p
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if userDir != "" {
		entries, err := os.ReadDir(userDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || (filepath.Ext(e.Name()) != ".yml" && filepath.Ext(e.Name()) != ".yaml") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(userDir, e.Name()))
			if err != nil {
				return nil, err
			}
			var p AgentPreset
			if err := yaml.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			if p.Name != "" {
				byName[p.Name] = p
			}
		}
	}
	out := make([]AgentPreset, 0, len(byName))
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
