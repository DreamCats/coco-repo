package cmd

import "embed"

const (
	embeddedSkillsRoot     = "embedded_skills"
	skillsManifestFileName = ".coco-ext-manifest.json"
)

// embeddedSkillsFS 内置的 skills 资源，供 install/uninstall 在无源码仓库场景下使用。
//
//go:embed embedded_skills/*/SKILL.md
var embeddedSkillsFS embed.FS
