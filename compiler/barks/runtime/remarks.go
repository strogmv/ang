package barks

// StoryRemark is an authored fact line offered by the host after checking who knows it.
type StoryRemark struct {
	Fact, Key string
	Cells     int
	Measure   map[string]Measure
}

func (m Model) storyRemark(s *State, f Frame, actors []Actor, byID map[string]Actor, day int64) (Speech, bool) {
	if f.Remark == nil {
		return Speech{}, false
	}
	type candidate struct {
		speaker Actor
		remark  StoryRemark
		pack    *Pack
		clock   PackClock
	}
	var candidates []candidate
	for _, a := range actors {
		if a.ID == f.Hero.ID || s.speaking(a.ID, f.Tick) {
			continue
		}
		remark, ok := f.Remark(a)
		if !ok || !m.hears(f.Hero, a, remark.Cells) {
			continue
		}
		item := candidate{speaker: a, remark: remark}
		if line, pack, ok := m.Line(remark.Key); ok {
			clock := s.Packs[pack.Place]
			if hour := f.Now / 3600; clock.Hour != hour {
				clock.Hour, clock.Count = hour, 0
			}
			if f.Tick < clock.Busy || clock.Last > 0 && f.Now < clock.Last+pack.GapSeconds || clock.Count >= pack.MaxPerHour {
				continue
			}
			// A migrated pack line keeps its period, post, place and line cooldown.
			place, found := m.Place(pack.Place)
			if !found || m.placeOf(a) != place.ID || !m.inPeriod(*pack, line.When, day) ||
				!m.requires(*line, a, byID, f, s) || !m.reacts(s, *line, place, a.ID, f) {
				continue
			}
			if last, said := s.Lines[line.ID]; said && f.Now < last+line.CooldownSeconds {
				continue
			}
			item.remark.Measure = line.Measure
			item.pack, item.clock = pack, clock
		}
		candidates = append(candidates, item)
	}
	if len(candidates) == 0 {
		return Speech{}, false
	}
	c := candidates[Roll(f.Seed, "barks.story", f.Hero.Level, f.Tick)%uint64(len(candidates))]
	caste := c.speaker.Caste
	if caste == "" {
		caste = m.Castes[c.speaker.ID]
	}
	face, _ := m.Face(caste)
	s.Serial++
	speech := Speech{Serial: s.Serial, Kind: "fact", Line: c.remark.Key, Key: c.remark.Key,
		Speaker: c.speaker.ID, Caste: caste, Second: f.Now, StoryFact: c.remark.Fact, HearingCells: c.remark.Cells}
	m.clock(&speech, face, c.remark.Measure, nil, f.Tick)
	s.Active = append(s.Active, speech)
	s.Talk.Last = f.Now
	s.Talk.Count++
	if c.pack != nil {
		c.clock.Last, c.clock.Busy = f.Now, speech.Until
		c.clock.Count++
		s.Packs[c.pack.Place], s.Lines[speech.Line] = c.clock, f.Now
	}
	return speech, true
}
