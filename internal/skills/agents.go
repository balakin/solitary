package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// agents is the skills CLI's own list (vercel-labs/skills, src/agents.ts at
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
var agents = []agent{
	{"AiderDesk", "~/.aider-desk/skills", []string{"~/.aider-desk"}},
	{"Amp", "$CONFIG/agents/skills", []string{"$CONFIG/amp"}},
	{"Antigravity", "~/.gemini/antigravity/skills", []string{"~/.gemini/antigravity"}},
	{"Antigravity CLI", "~/.gemini/antigravity-cli/skills", []string{"~/.gemini/antigravity-cli"}},
	{"AstrBot", "~/.astrbot/data/skills", []string{"~/.astrbot"}},
	{"Autohand Code CLI", "$AUTOHAND_HOME/skills", []string{"$AUTOHAND_HOME"}},
	{"Augment", "~/.augment/skills", []string{"~/.augment"}},
	{"IBM Bob", "~/.bob/skills", []string{"~/.bob"}},
	{"Claude Code", "$CLAUDE_CONFIG_DIR/skills", []string{"$CLAUDE_CONFIG_DIR"}},
	// Upstream installs into whichever of its three past names exists, in this
	// order; the first one found wins, since one agent is one target.
	{"OpenClaw", "~/.openclaw/skills", []string{"~/.openclaw"}},
	{"OpenClaw", "~/.clawdbot/skills", []string{"~/.clawdbot"}},
	{"OpenClaw", "~/.moltbot/skills", []string{"~/.moltbot"}},
	{"Cline", "~/.agents/skills", []string{"~/.cline"}},
	{"CodeArts Agent", "~/.codeartsdoer/skills", []string{"~/.codeartsdoer"}},
	{"CodeBuddy", "~/.codebuddy/skills", []string{"~/.codebuddy"}},
	{"Codemaker", "~/.codemaker/skills", []string{"~/.codemaker"}},
	{"Code Studio", "~/.codestudio/skills", []string{"~/.codestudio"}},
	{"Codex", "~/.agents/skills", []string{"$CODEX_HOME", "/etc/codex"}},
	{"Command Code", "~/.commandcode/skills", []string{"~/.commandcode"}},
	{"Continue", "~/.continue/skills", []string{"~/.continue"}},
	{"Cortex Code", "~/.snowflake/cortex/skills", []string{"~/.snowflake/cortex"}},
	{"Crush", "~/.config/crush/skills", []string{"~/.config/crush"}},
	{"Cursor", "~/.cursor/skills", []string{"~/.cursor"}},
	{"Deep Agents", "~/.deepagents/agent/skills", []string{"~/.deepagents"}},
	{"Devin for Terminal", "$CONFIG/devin/skills", []string{"$CONFIG/devin"}},
	{"Dexto", "~/.agents/skills", []string{"~/.dexto"}},
	{"Droid", "~/.factory/skills", []string{"~/.factory"}},
	{"Firebender", "~/.firebender/skills", []string{"~/.firebender"}},
	{"ForgeCode", "~/.forge/skills", []string{"~/.forge"}},
	{"fx", "~/.agents/skills", []string{"~/.fx"}},
	{"Gemini CLI", "~/.gemini/skills", []string{"~/.gemini"}},
	{"GitHub Copilot", "~/.copilot/skills", []string{"~/.copilot"}},
	{"Goose", "$CONFIG/goose/skills", []string{"$CONFIG/goose"}},
	{"Grok Build", "$GROK_HOME/skills", []string{"$GROK_HOME"}},
	{"Hermes Agent", "$HERMES_HOME/skills", []string{"$HERMES_HOME"}},
	{"inference.sh", "~/.inferencesh/skills", []string{"~/.inferencesh"}},
	{"Jazz", "~/.jazz/skills", []string{"~/.jazz"}},
	{"Junie", "~/.junie/skills", []string{"~/.junie"}},
	{"iFlow CLI", "~/.iflow/skills", []string{"~/.iflow"}},
	{"Kilo Code", "~/.kilo/skills", []string{"~/.kilo", "~/.kilocode"}},
	{"Kimchi", "~/.config/kimchi/harness/skills", []string{"~/.config/kimchi"}},
	{"Kimi Code CLI", "~/.agents/skills", []string{"~/.kimi-code", "~/.kimi"}},
	{"Kiro CLI", "~/.kiro/skills", []string{"~/.kiro"}},
	{"Kode", "~/.kode/skills", []string{"~/.kode"}},
	{"Lingma", "~/.lingma/skills", []string{"~/.lingma"}},
	{"Loaf", "~/.agents/skills", []string{"~/.loaf"}},
	{"MCPJam", "~/.mcpjam/skills", []string{"~/.mcpjam"}},
	{"MiniMax Code", "~/.minimax/skills", []string{"~/.minimax", "/Applications/MiniMax Code.app"}},
	{"Mistral Vibe", "$VIBE_HOME/skills", []string{"$VIBE_HOME"}},
	{"Moxby", "~/.moxby/skills", []string{"~/.moxby"}},
	{"Mux", "~/.mux/skills", []string{"~/.mux"}},
	{"OpenCode", "$CONFIG/opencode/skills", []string{"$CONFIG/opencode"}},
	{"OpenHands", "~/.openhands/skills", []string{"~/.openhands"}},
	{"Ona", "~/.ona/skills", []string{"~/.ona"}},
	{"Pi", "~/.agents/skills", []string{"~/.pi/agent"}},
	{"Posit Assistant", "~/.posit/assistant/skills", []string{"~/.posit/assistant", "~/.positai"}},
	{"Qoder", "~/.qoder/skills", []string{"~/.qoder"}},
	{"Qoder CN", "~/.qoder-cn/skills", []string{"~/.qoder-cn"}},
	{"Qwen Code", "~/.qwen/skills", []string{"~/.qwen"}},
	{"Reasonix", "~/.reasonix/skills", []string{"~/.reasonix"}},
	{"Rovo Dev", "~/.rovodev/skills", []string{"~/.rovodev"}},
	{"Roo Code", "~/.roo/skills", []string{"~/.roo"}},
	{"Sarvam Code", "~/.agents/skills", []string{"$SARVAM_HOME"}},
	{"Tabnine CLI", "~/.tabnine/agent/skills", []string{"~/.tabnine"}},
	{"Terramind", "~/.terramind/skills", []string{"~/.terramind"}},
	{"Tinycloud", "~/.tinycloud/skills", []string{"~/.tinycloud"}},
	{"Trae", "~/.trae/skills", []string{"~/.trae"}},
	{"Trae CN", "~/.trae-cn/skills", []string{"~/.trae-cn"}},
	{"Warp", "~/.agents/skills", []string{"~/.warp"}},
	{"Windsurf", "~/.codeium/windsurf/skills", []string{"~/.codeium/windsurf"}},
	{"Zed", "~/.agents/skills", []string{"$CONFIG/zed", "$FLATPAK_XDG_CONFIG_HOME/zed"}},
	{"ZCode", "~/.zcode/skills", []string{"~/.zcode", "/Applications/ZCode.app"}},
	{"Zencoder", "~/.zencoder/skills", []string{"~/.zencoder"}},
	{"Zenflow", "~/.zencoder/skills", []string{"~/.zencoder"}},
	{"Neovate", "~/.neovate/skills", []string{"~/.neovate"}},
	{"Pochi", "~/.pochi/skills", []string{"~/.pochi"}},
	{"AdaL", "~/.adal/skills", []string{"~/.adal"}},
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

// system is where an absolute marker is looked for, and is only ever moved by a
// test: a marker like /etc/codex is a fact about the machine running it.
var system = "/"

type agent struct {
	name    string
	skills  string
	markers []string
}

// resolve turns a path as the list writes it into one on this machine, or ""
// when it is built on a variable that is unset and has no default.
func resolve(home, path string) string {
	head, rest, _ := strings.Cut(path, "/")

	switch {
	case head == "~":
		return filepath.Join(home, rest)
	case head == "$CONFIG":
		if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
			return filepath.Join(dir, rest)
		}
		return filepath.Join(home, ".config", rest)
	case strings.HasPrefix(head, "$"):
		if dir := strings.TrimSpace(os.Getenv(head[1:])); dir != "" {
			return filepath.Join(dir, rest)
		}
		if def, ok := homes[head[1:]]; ok {
			return resolve(home, filepath.ToSlash(filepath.Join(def, rest)))
		}
		return ""
	default:
		return filepath.Join(system, path)
	}
}
