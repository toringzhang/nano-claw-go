package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_readSkill(t *testing.T) {
	skill, err := readSkill("skills/skill-creator")
	assert.Nil(t, err)
	assert.Equal(t, "skill-creator", skill.Name)
	assert.Equal(t, "description", skill.Description)
}

func Test_SkillLoader(t *testing.T) {
	loader := NewSkillLoader("skills")
	err := loader.Load()
	assert.Nil(t, err)
	t.Logf("prompt: %s", loader.Prompt())
	t.Logf("my-skills: %s", loader.Skill("pdf-processing"))
}
