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

// system is where an absolute marker is looked for, and is only ever moved by a
// test: a marker like /etc/codex is a fact about the machine running it.
var system = "/"

type agent struct {
	id      string // what --agent takes, as `npx skills add --agent` does
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
