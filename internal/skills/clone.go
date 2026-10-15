package skills

import "maps"

// Clone returns an owned copy of a registered skill.
func Clone(s *Skill) *Skill {
	if s == nil {
		return nil
	}
	copied := *s
	copied.Metadata = maps.Clone(s.Metadata)
	copied.Tags = append([]string(nil), s.Tags...)
	return &copied
}

// CloneAll copies a catalogue, including its mutable fields.
func CloneAll(list []*Skill) []*Skill {
	if list == nil {
		return nil
	}
	out := make([]*Skill, len(list))
	for i, s := range list {
		out[i] = Clone(s)
	}
	return out
}
