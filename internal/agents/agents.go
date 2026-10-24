// Package agents is the coding agents solitary knows how to give skills to:
// what each is called, where it reads skills from, and what says it is there.
// The host installer and a cell's definition both name agents from it.
package agents

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// list is the skills CLI's own list (vercel-labs/skills, src/agents.ts at
// a80e4c9), so that `solitary skills install` finds what `npx skills add -g`
// would without needing node. Only the global half is kept: an agent detected
// by something in the current directory, or with no global skills directory at
// all (Eve, PromptScript, Replit), has nowhere in a home to install to.
//
// Paths are written as upstream builds them. "~" is the home directory,
// "$CONFIG" is XDG_CONFIG_HOME or ~/.config on every platform — which is what
// the agents that live there use, macOS included — and "$NAME" is an override
// the agent itself honours, falling back to the default in homes. An agent is
// configured when any one of its markers exists.
//
// Refreshing it means reading the same file again: the order is upstream's, so
// a diff against it stays readable.
var list = []Agent{
	{"aider-desk", "AiderDesk", "~/.aider-desk/skills", []string{"~/.aider-desk"}},
	{"amp", "Amp", "$CONFIG/agents/skills", []string{"$CONFIG/amp"}},
	{"antigravity", "Antigravity", "~/.gemini/antigravity/skills", []string{"~/.gemini/antigravity"}},
	{"antigravity-cli", "Antigravity CLI", "~/.gemini/antigravity-cli/skills", []string{"~/.gemini/antigravity-cli"}},
	{"astrbot", "AstrBot", "~/.astrbot/data/skills", []string{"~/.astrbot"}},
	{"autohand-code", "Autohand Code CLI", "$AUTOHAND_HOME/skills", []string{"$AUTOHAND_HOME"}},
	{"augment", "Augment", "~/.augment/skills", []string{"~/.augment"}},
	{"bob", "IBM Bob", "~/.bob/skills", []string{"~/.bob"}},
	{"claude-code", "Claude Code", "$CLAUDE_CONFIG_DIR/skills", []string{"$CLAUDE_CONFIG_DIR"}},
	// Upstream installs into whichever of its three past names exists, in this
	// order; the first one found wins, since one agent is one target.
	{"openclaw", "OpenClaw", "~/.openclaw/skills", []string{"~/.openclaw"}},
	{"openclaw", "OpenClaw", "~/.clawdbot/skills", []string{"~/.clawdbot"}},
	{"openclaw", "OpenClaw", "~/.moltbot/skills", []string{"~/.moltbot"}},
	{"cline", "Cline", "~/.agents/skills", []string{"~/.cline"}},
	{"codearts-agent", "CodeArts Agent", "~/.codeartsdoer/skills", []string{"~/.codeartsdoer"}},
	{"codebuddy", "CodeBuddy", "~/.codebuddy/skills", []string{"~/.codebuddy"}},
	{"codemaker", "Codemaker", "~/.codemaker/skills", []string{"~/.codemaker"}},
	{"codestudio", "Code Studio", "~/.codestudio/skills", []string{"~/.codestudio"}},
	{"codex", "Codex", "~/.agents/skills", []string{"$CODEX_HOME", "/etc/codex"}},
	{"command-code", "Command Code", "~/.commandcode/skills", []string{"~/.commandcode"}},
	{"continue", "Continue", "~/.continue/skills", []string{"~/.continue"}},
	{"cortex", "Cortex Code", "~/.snowflake/cortex/skills", []string{"~/.snowflake/cortex"}},
	{"crush", "Crush", "~/.config/crush/skills", []string{"~/.config/crush"}},
	{"cursor", "Cursor", "~/.cursor/skills", []string{"~/.cursor"}},
	{"deepagents", "Deep Agents", "~/.deepagents/agent/skills", []string{"~/.deepagents"}},
	{"devin", "Devin for Terminal", "$CONFIG/devin/skills", []string{"$CONFIG/devin"}},
	{"dexto", "Dexto", "~/.agents/skills", []string{"~/.dexto"}},
	{"droid", "Droid", "~/.factory/skills", []string{"~/.factory"}},
	{"firebender", "Firebender", "~/.firebender/skills", []string{"~/.firebender"}},
	{"forgecode", "ForgeCode", "~/.forge/skills", []string{"~/.forge"}},
	{"fx", "fx", "~/.agents/skills", []string{"~/.fx"}},
	{"gemini-cli", "Gemini CLI", "~/.gemini/skills", []string{"~/.gemini"}},
	{"github-copilot", "GitHub Copilot", "~/.copilot/skills", []string{"~/.copilot"}},
	{"goose", "Goose", "$CONFIG/goose/skills", []string{"$CONFIG/goose"}},
	{"grok", "Grok Build", "$GROK_HOME/skills", []string{"$GROK_HOME"}},
	{"hermes-agent", "Hermes Agent", "$HERMES_HOME/skills", []string{"$HERMES_HOME"}},
	{"inference-sh", "inference.sh", "~/.inferencesh/skills", []string{"~/.inferencesh"}},
	{"jazz", "Jazz", "~/.jazz/skills", []string{"~/.jazz"}},
	{"junie", "Junie", "~/.junie/skills", []string{"~/.junie"}},
	{"iflow-cli", "iFlow CLI", "~/.iflow/skills", []string{"~/.iflow"}},
	{"kilo", "Kilo Code", "~/.kilo/skills", []string{"~/.kilo", "~/.kilocode"}},
	{"kimchi", "Kimchi", "~/.config/kimchi/harness/skills", []string{"~/.config/kimchi"}},
	{"kimi-code-cli", "Kimi Code CLI", "~/.agents/skills", []string{"~/.kimi-code", "~/.kimi"}},
	{"kiro-cli", "Kiro CLI", "~/.kiro/skills", []string{"~/.kiro"}},
	{"kode", "Kode", "~/.kode/skills", []string{"~/.kode"}},
	{"lingma", "Lingma", "~/.lingma/skills", []string{"~/.lingma"}},
	{"loaf", "Loaf", "~/.agents/skills", []string{"~/.loaf"}},
	{"mcpjam", "MCPJam", "~/.mcpjam/skills", []string{"~/.mcpjam"}},
	{"minimax-code", "MiniMax Code", "~/.minimax/skills", []string{"~/.minimax", "/Applications/MiniMax Code.app"}},
	{"mistral-vibe", "Mistral Vibe", "$VIBE_HOME/skills", []string{"$VIBE_HOME"}},
	{"moxby", "Moxby", "~/.moxby/skills", []string{"~/.moxby"}},
	{"mux", "Mux", "~/.mux/skills", []string{"~/.mux"}},
	{"opencode", "OpenCode", "$CONFIG/opencode/skills", []string{"$CONFIG/opencode"}},
	{"openhands", "OpenHands", "~/.openhands/skills", []string{"~/.openhands"}},
	{"ona", "Ona", "~/.ona/skills", []string{"~/.ona"}},
	{"pi", "Pi", "~/.agents/skills", []string{"~/.pi/agent"}},
	{"posit-assistant", "Posit Assistant", "~/.posit/assistant/skills", []string{"~/.posit/assistant", "~/.positai"}},
	{"qoder", "Qoder", "~/.qoder/skills", []string{"~/.qoder"}},
	{"qoder-cn", "Qoder CN", "~/.qoder-cn/skills", []string{"~/.qoder-cn"}},
	{"qwen-code", "Qwen Code", "~/.qwen/skills", []string{"~/.qwen"}},
	{"reasonix", "Reasonix", "~/.reasonix/skills", []string{"~/.reasonix"}},
	{"rovodev", "Rovo Dev", "~/.rovodev/skills", []string{"~/.rovodev"}},
	{"roo", "Roo Code", "~/.roo/skills", []string{"~/.roo"}},
	{"sarvam-code", "Sarvam Code", "~/.agents/skills", []string{"$SARVAM_HOME"}},
	{"tabnine-cli", "Tabnine CLI", "~/.tabnine/agent/skills", []string{"~/.tabnine"}},
	{"terramind", "Terramind", "~/.terramind/skills", []string{"~/.terramind"}},
	{"tinycloud", "Tinycloud", "~/.tinycloud/skills", []string{"~/.tinycloud"}},
	{"trae", "Trae", "~/.trae/skills", []string{"~/.trae"}},
	{"trae-cn", "Trae CN", "~/.trae-cn/skills", []string{"~/.trae-cn"}},
	{"warp", "Warp", "~/.agents/skills", []string{"~/.warp"}},
	{"windsurf", "Windsurf", "~/.codeium/windsurf/skills", []string{"~/.codeium/windsurf"}},
	{"zed", "Zed", "~/.agents/skills", []string{"$CONFIG/zed", "$FLATPAK_XDG_CONFIG_HOME/zed"}},
	{"zcode", "ZCode", "~/.zcode/skills", []string{"~/.zcode", "/Applications/ZCode.app"}},
	{"zencoder", "Zencoder", "~/.zencoder/skills", []string{"~/.zencoder"}},
	{"zenflow", "Zenflow", "~/.zencoder/skills", []string{"~/.zencoder"}},
	{"neovate", "Neovate", "~/.neovate/skills", []string{"~/.neovate"}},
	{"pochi", "Pochi", "~/.pochi/skills", []string{"~/.pochi"}},
	{"adal", "AdaL", "~/.adal/skills", []string{"~/.adal"}},
}

// homes are the defaults of the overrides agents honour. A variable missing
// from here has no default: a marker built on it counts only while it is set.
var homes = map[string]string{
	"CLAUDE_CONFIG_DIR": "~/.claude",
	"CODEX_HOME":        "~/.codex",
	"VIBE_HOME":         "~/.vibe",
	"HERMES_HOME":       "~/.hermes",
	"AUTOHAND_HOME":     "~/.autohand",
	"GROK_HOME":         "~/.grok",
	"SARVAM_HOME":       "~/.sarvam",
}

// Agent is one coding agent and where it keeps its skills.
type Agent struct {
	ID      string // what --agent and a cell's agents: take, as `npx skills add --agent` does
	Name    string
	Skills  string
	Markers []string
}

// All is every agent, in the list's order. An agent with more than one possible
// directory appears once for each, in the order they are preferred.
func All() []Agent {
	return slices.Clone(list)
}

// IDs lists every agent ID once, in the list's order.
func IDs() []string {
	var ids []string
	for _, agent := range list {
		if !slices.Contains(ids, agent.ID) {
			ids = append(ids, agent.ID)
		}
	}
	return ids
}

// Known reports whether id names an agent on the list.
func Known(id string) bool {
	return slices.ContainsFunc(list, func(a Agent) bool { return a.ID == id })
}

// Vars are the environment variables the list's paths are built on, so that a
// caller resolving them somewhere else — inside a container — knows what to ask
// that place for.
func Vars() []string {
	vars := []string{"XDG_CONFIG_HOME"}
	for _, agent := range list {
		for _, path := range append([]string{agent.Skills}, agent.Markers...) {
			head, _, _ := strings.Cut(path, "/")
			if strings.HasPrefix(head, "$") && head != "$CONFIG" && !slices.Contains(vars, head[1:]) {
				vars = append(vars, head[1:])
			}
		}
	}
	return vars
}

// Expand turns a path as the list writes it into one under home, reading
// variables through getenv, or "" when it is built on a variable that is unset
// and has no default. An absolute path is returned as it is: where it is looked
// for is the caller's business.
func Expand(path, home string, getenv func(string) string) string {
	head, rest, _ := strings.Cut(path, "/")

	switch {
	case head == "~":
		return filepath.Join(home, rest)
	case head == "$CONFIG":
		if dir := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); dir != "" {
			return filepath.Join(dir, rest)
		}
		return filepath.Join(home, ".config", rest)
	case strings.HasPrefix(head, "$"):
		if dir := strings.TrimSpace(getenv(head[1:])); dir != "" {
			return filepath.Join(dir, rest)
		}
		if def, ok := homes[head[1:]]; ok {
			return Expand(filepath.ToSlash(filepath.Join(def, rest)), home, getenv)
		}
		return ""
	default:
		return path
	}
}

// Target is one directory skills are installed into.
type Target struct {
	Agents []string // empty for a directory named rather than found
	Dir    string
}

// Place is where agents are looked for: how a path from the list becomes a real
// one there, and whether it exists. The host and a cell's container are each a
// place, with their own home and their own environment.
type Place struct {
	Expand func(path string) string
	Exists func(path string) bool
}

// Detect returns the skills directory of every agent configured in place.
// Agents reading the same directory are one target, so a skill is written once
// however many of them will read it.
func Detect(place Place) []Target {
	var found []Target
	var seen []string
	for _, agent := range list {
		if slices.Contains(seen, agent.ID) || !configured(place, agent) {
			continue
		}
		seen = append(seen, agent.ID)
		found = addTarget(found, agent.Name, place.Expand(agent.Skills))
	}
	return found
}

// Named returns the skills directory of each agent named by its ID, whether or
// not it is configured yet: naming one is how an agent is installed for before
// it has ever run. An agent with more than one directory gets the first one
// that is configured, or else the first.
func Named(place Place, ids []string) ([]Target, error) {
	var found []Target
	for _, id := range ids {
		agent, err := pick(place, id)
		if err != nil {
			return nil, err
		}
		found = addTarget(found, agent.Name, place.Expand(agent.Skills))
	}
	return found, nil
}

// Unknown is the error for an ID that is not on the list.
func Unknown(id string) error {
	return fmt.Errorf("unknown agent %q; known agents are: %s", id, strings.Join(slices.Sorted(slices.Values(IDs())), ", "))
}

func pick(place Place, id string) (Agent, error) {
	var match *Agent
	for i := range list {
		if list[i].ID != id {
			continue
		}
		if match == nil {
			match = &list[i]
		}
		if configured(place, list[i]) {
			return list[i], nil
		}
	}
	if match == nil {
		return Agent{}, Unknown(id)
	}
	return *match, nil
}

func configured(place Place, agent Agent) bool {
	for _, marker := range agent.Markers {
		if path := place.Expand(marker); path != "" && place.Exists(path) {
			return true
		}
	}
	return false
}

func addTarget(found []Target, name, dir string) []Target {
	if dir == "" {
		return found
	}
	if i := slices.IndexFunc(found, func(t Target) bool { return t.Dir == dir }); i >= 0 {
		if !slices.Contains(found[i].Agents, name) {
			found[i].Agents = append(found[i].Agents, name)
		}
		return found
	}
	return append(found, Target{Agents: []string{name}, Dir: dir})
}
