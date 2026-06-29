package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/skills"
)

var skillCatalog *skills.Catalog

// SetSkillCatalog wires the skill index for list_skills / load_skill tools.
func SetSkillCatalog(c *skills.Catalog) {
	skillCatalog = c
}

type listSkillsTool struct{}

func (t *listSkillsTool) Name() string        { return "list_skills" }
func (t *listSkillsTool) Description() string { return "List available agent skills (SKILL.md) and short descriptions." }
func (t *listSkillsTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *listSkillsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{}
}
func (t *listSkillsTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	if skillCatalog == nil || skillCatalog.Count() == 0 {
		return OkResult("No skills loaded. Add SKILL.md under ~/.omnidev-agent/skills/ or .omnidev-agent/skills/")
	}
	var sb strings.Builder
	for _, sk := range skillCatalog.List() {
		sb.WriteString(fmt.Sprintf("- %s: %s (%s)\n", sk.Name, sk.Description, sk.Path))
	}
	return okLimited("list_skills", strings.TrimSpace(sb.String()))
}

type loadSkillTool struct{}

func (t *loadSkillTool) Name() string { return "load_skill" }
func (t *loadSkillTool) Description() string {
	return "Load a skill by name. Returns full SKILL.md instructions — follow them for the current task."
}
func (t *loadSkillTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *loadSkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"description": "Skill name (from list_skills).",
			"required":    true,
		},
	}
}
func (t *loadSkillTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	name := getStringArg(args, "name", "")
	if name == "" {
		return ErrResult("name is required")
	}
	if skillCatalog == nil {
		return ErrResult("skill catalog not initialized")
	}
	sk, ok := skillCatalog.Get(name)
	if !ok {
		return ErrResult("skill not found: " + name + " (use list_skills)")
	}
	out := fmt.Sprintf("[SKILL: %s]\n%s\n\n(Follow the skill instructions above.)", sk.Name, sk.Body)
	return okLimited("load_skill", out)
}
