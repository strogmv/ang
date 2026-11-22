package gamepoc

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type heroFileUsageSource struct {
	Partial       bool     `json:"partial"`
	Conversations []string `json:"conversations,omitempty"`
	Interjections []string `json:"interjections,omitempty"`
}

// addDialogue runs before the skill and episode gates. A script that has moved
// completely must have no Markdown counts. Partial sources add to the remaining rows.
func (u *HeroFileUsage) addDialogue(c *Compiled) error {
	if len(u.Dialogue) == 0 {
		return nil
	}
	interjections := map[string]Interjection{}
	if c.Memory != nil {
		for _, in := range c.Memory.Interjections {
			interjections[in.ID] = in
		}
	}
	owners := map[string]string{}
	claim := func(id, number string) error {
		if owner := owners[id]; owner != "" {
			return fmt.Errorf("%s counted twice (scripts %s and %s)", id, owner, number)
		}
		owners[id] = number
		return nil
	}
	for _, number := range sortedKeys(u.Dialogue) {
		source := u.Dialogue[number]
		err := func() error {
			if !slices.Contains(u.ActOne, number) || len(source.Conversations)+len(source.Interjections) == 0 {
				return fmt.Errorf("expected an act-one number with at least one CUE source")
			}
			_, hasCounts := u.Counts[number]
			_, hasReads := u.Reads[number]
			if hasCounts != source.Partial || hasReads != source.Partial {
				return fmt.Errorf("Markdown contribution must be present only for a partial transfer")
			}
			uses := dialogueUsage{reads: map[string]int{}}
			for _, id := range source.Conversations {
				if err := claim(id, number); err != nil {
					return err
				}
				conv := c.Conversations[id]
				if conv == nil || conv.Waits != "" {
					return fmt.Errorf("no live conversation %s", id)
				}
				uses.conversation(conv)
			}
			// One scripted inner-voice row can follow several NPCs' interjections.
			voices := map[string]Interjection{}
			for _, id := range source.Interjections {
				if err := claim(id, number); err != nil {
					return err
				}
				in, ok := interjections[id]
				if !ok || in.Waits != "" {
					return fmt.Errorf("no live interjection %s", id)
				}
				uses.read(in.Fact)
				uses.when(in.When)
				uses.effects(in.Do)
				if v := in.Voice; v != nil {
					first, repeated := voices[v.Text]
					if repeated {
						if !reflect.DeepEqual(first.Voice, v) {
							return fmt.Errorf("conflicting versions of interjection voice %s", v.Text)
						}
					} else {
						if err := claim(v.Text, number); err != nil {
							return err
						}
						voices[v.Text] = in
						uses.voices = append(uses.voices, v.Skill)
						uses.when(v.When)
					}
				}
			}
			if u.Counts[number] == nil {
				u.Counts[number] = make(map[string]struct {
					Checks int `json:"checks"`
					Voices int `json:"voices"`
				})
			}
			if u.Reads[number] == nil {
				u.Reads[number] = map[string]int{}
			}
			for _, skill := range uses.checks {
				count := u.Counts[number][skill]
				count.Checks++
				u.Counts[number][skill] = count
			}
			for _, skill := range uses.voices {
				count := u.Counts[number][skill]
				count.Voices++
				u.Counts[number][skill] = count
			}
			for id, count := range uses.reads {
				u.Reads[number][id] += count
			}
			return nil
		}()
		if err != nil {
			return fmt.Errorf("world.hero_file.usage.dialogue.%s: %w", number, err)
		}
	}
	return nil
}

type dialogueUsage struct {
	checks, voices []string
	reads          map[string]int
}

func (u *dialogueUsage) read(id string) {
	if strings.HasPrefix(id, "topic.") || strings.HasPrefix(id, "fact.file.") {
		u.reads[id]++
	}
}

func (u *dialogueUsage) when(w *When) {
	if w == nil {
		return
	}
	if w.Fact != nil {
		u.read(w.Fact.ID)
	}
	if w.Knows != nil {
		u.read(w.Knows.Fact)
	}
	u.read(w.NotFact)
	u.read(w.Topic)
	u.read(w.NotTopic)
	for i := range w.Any {
		u.when(&w.Any[i])
	}
	u.when(w.Not)
}

func (u *dialogueUsage) effects(effects []Do) {
	for _, d := range effects {
		u.when(d.When)
		u.read(d.Learn)
	}
}

func (u *dialogueUsage) variants(variants []ConvVariant) {
	for i := range variants {
		u.when(&variants[i].When)
	}
}

func (u *dialogueUsage) lines(lines []ConvLine) {
	for _, l := range lines {
		u.when(l.When)
		u.variants(l.Variants)
	}
}

func (u *dialogueUsage) conversation(c *Conversation) {
	for _, entry := range c.Entries {
		u.when(entry.When)
		u.lines(entry.Lines)
		u.effects(entry.Do)
	}
	for _, node := range c.Nodes {
		u.lines(node.Lines)
		u.effects(node.Do)
		for _, voice := range node.Voices {
			u.voices = append(u.voices, voice.Skill)
			u.when(voice.When)
			u.variants(voice.Variants)
			u.effects(voice.Do)
		}
		for _, option := range node.Options {
			if option.Check != nil {
				u.checks = append(u.checks, option.Check.Skill)
			}
			u.when(option.When)
			u.variants(option.Variants)
			u.lines(option.Reply)
			u.effects(option.Do)
			if option.Fail != nil {
				u.lines(option.Fail.Reply)
				u.effects(option.Fail.Do)
			}
		}
	}
	for _, event := range c.Events {
		u.lines(event.Lines)
		u.effects(event.Do)
	}
}
