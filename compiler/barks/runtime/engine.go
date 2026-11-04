package barks

import (
	"sort"
	"strings"
)

// Actor is a person of the hero's level as the host sees him this frame.
type Actor struct {
	ID     string `json:"id"`
	Level  string `json:"level"`
	At     Cell   `json:"at"`
	Seated bool   `json:"seated,omitempty"`  // on a seat of a brain place (a yatai stool)
	OnPost bool   `json:"on_post,omitempty"` // on his post: a staff spot, a counter post, the master's counter
	Caste  string `json:"caste,omitempty"`   // an anonymous person of the population (his profile's caste)
	Moving bool   `json:"moving,omitempty"`  // on his way somewhere: he does not stop to talk
}

// Fact is a world fact holding this frame. Since > 0 is when it happened (a raid at 21:00, a refused rebinding);
// Since 0 = a state that holds now (a gate locked, a lane queued): the engine remembers its onset.
type Fact struct {
	Event   string `json:"event"`
	Subject string `json:"subject,omitempty"`
	Level   string `json:"level,omitempty"`
	Since   int64  `json:"since,omitempty"`
}

// Frame is the world the engine reads: the game clock, the tick, the seed, the hero and the people of his level.
type Frame struct {
	Now    int64
	Tick   int64
	Seed   int64
	Hero   Actor
	Actors []Actor
	Facts  []Fact
}

func factKey(f Fact) string { return f.Event + "|" + f.Subject + "|" + f.Level }

// Step advances the engine by one host frame and returns the speeches started in it.
func (m Model) Step(s *State, f Frame) []Speech {
	if s.Lines == nil {
		s.Lines = map[string]int64{}
	}
	if s.Packs == nil {
		s.Packs = map[string]PackClock{}
	}
	elapsed := f.Now - s.Now
	if elapsed < 0 {
		elapsed = 0
	}
	if s.Now == 0 {
		elapsed = 0
	}
	s.Now = f.Now
	// the hero's last step
	if s.Hero.Level != f.Hero.Level || s.Hero.At != f.Hero.At {
		if s.Hero.Level != "" {
			s.Hero.Moved = f.Tick
		}
		s.Hero.Level, s.Hero.At = f.Hero.Level, f.Hero.At
	}
	// fact onsets: a state fact starts when first seen, an event fact carries its own time; a fact gone is forgotten
	seen := map[string]bool{}
	next := map[string]int64{}
	for _, fact := range f.Facts {
		k := factKey(fact)
		seen[k] = true
		switch {
		case fact.Since > 0:
			next[k] = fact.Since
		case s.Facts[k] > 0:
			next[k] = s.Facts[k]
		default:
			next[k] = f.Now
		}
	}
	s.Facts = next
	if len(s.Facts) == 0 {
		s.Facts = nil
	}
	// lines that are over
	keep := s.Active[:0]
	for _, a := range s.Active {
		if a.Until > f.Tick {
			keep = append(keep, a)
		}
	}
	s.Active = keep
	if len(s.Active) == 0 {
		s.Active = nil
	}
	actors := append([]Actor(nil), f.Actors...)
	sort.Slice(actors, func(i, j int) bool { return actors[i].ID < actors[j].ID })
	byID := map[string]Actor{}
	for _, a := range actors {
		byID[a.ID] = a
	}
	started := []Speech{}
	// answers due
	pending := s.Pending[:0]
	for _, p := range s.Pending {
		if f.Now < p.Second || f.Tick < p.Tick {
			pending = append(pending, p)
			continue
		}
		r, ok := m.Reply(p.Reply)
		speaker, here := byID[r.Speaker]
		pack, _ := m.Pack(p.Place)
		if !ok || !here || pack == nil || !m.hears(f.Hero, speaker, pack.HeroWithinCells) || s.speaking(r.Speaker, f.Tick) {
			continue // the answer is lost: he left, or the hero walked away
		}
		sp := m.reply(s, *r, p, f)
		started = append(started, sp)
		c := s.Packs[p.Place]
		if sp.Until > c.Busy {
			c.Busy = sp.Until
		}
		s.Packs[p.Place] = c
	}
	s.Pending = pending
	if len(s.Pending) == 0 {
		s.Pending = nil
	}
	day := f.Now % 86400
	for i := range m.Packs {
		pack := &m.Packs[i]
		if s.audible(f.Tick) >= m.MaxActive {
			break
		}
		place, ok := m.Place(pack.Place)
		if !ok || place.Level != f.Hero.Level {
			continue
		}
		c := s.Packs[pack.Place]
		hour := f.Now / 3600
		if c.Hour != hour {
			c.Hour, c.Count = hour, 0
		}
		if f.Tick < c.Busy || (c.Last > 0 && f.Now < c.Last+pack.GapSeconds) || c.Count >= pack.MaxPerHour {
			continue
		}
		// the hazard: after the gap a place speaks on average once per MeanWaitSeconds of game time
		chance := elapsed * 1000000 / pack.MeanWaitSeconds
		if chance < 1000000 && int64(Roll(f.Seed, "barks.pack", pack.Place, f.Tick)%1000000) >= chance {
			continue
		}
		type cand struct {
			line    *Line
			speaker Actor
		}
		var cands []cand
		total := 0
		for j := range pack.Lines {
			l := &pack.Lines[j]
			if last, said := s.Lines[l.ID]; said && f.Now < last+l.CooldownSeconds {
				continue
			}
			if !m.inPeriod(*pack, l.When, day) {
				continue
			}
			for _, a := range m.speakersOf(*l, place, actors, f.Hero.ID) {
				if s.speaking(a.ID, f.Tick) || s.inTalk(a.ID) || !m.hears(f.Hero, a, pack.HeroWithinCells) || !m.requires(*l, a, byID, f, s) || !m.reacts(s, *l, place, a.ID, f) {
					continue
				}
				cands = append(cands, cand{l, a})
				total += l.Weight
				if l.Speaker != Anyone {
					break
				}
			}
		}
		if total == 0 {
			continue
		}
		pick := int(Roll(f.Seed, "barks.pick", pack.Place, f.Tick) % uint64(total))
		var chosen cand
		for _, c := range cands {
			if pick < c.line.Weight {
				chosen = c
				break
			}
			pick -= c.line.Weight
		}
		sp := m.say(s, *chosen.line, chosen.speaker.ID, pack.Place, f)
		started = append(started, sp)
		s.Lines[chosen.line.ID] = f.Now
		c.Last, c.Busy = f.Now, sp.Until
		c.Count++
		s.Packs[pack.Place] = c
		if chosen.line.Reply != "" {
			if r, ok := m.Reply(chosen.line.Reply); ok {
				due := sp.Until - m.ticks(m.Timing.FadeMS)
				s.Pending = append(s.Pending, Pending{Reply: r.ID, Place: pack.Place, Line: chosen.line.ID, Second: f.Now + r.DelaySeconds, Tick: due, Speaker: r.Speaker})
			}
		}
	}
	started = append(started, m.stepTalks(s, f, actors, byID, elapsed, day)...)
	return started
}

// inTalk: this person is in the middle of an exchange: he says nothing else between its turns.
func (s *State) inTalk(id string) bool {
	for _, run := range s.Talks {
		if run.A == id || run.B == id {
			return true
		}
	}
	return false
}

func postureFits(p Party, a Actor) bool {
	if a.Moving {
		return false
	}
	switch p.Posture {
	case "post":
		return a.OnPost
	case "seated":
		return a.Seated
	case "standing":
		return !a.Seated
	}
	return true
}

// talkersFit: both people where they must be, near each other; the hero hears the one who speaks.
func (m Model) talkersFit(e Exchange, a, b Actor) bool {
	return a.Level == b.Level && postureFits(e.A, a) && postureFits(e.B, b) && dist2(a.At, b.At) <= e.WithinCells*e.WithinCells
}

// stepTalks: the turns due of the exchanges in progress, then at most one new exchange (doc 94 §21, demo-hub v26-alive).
func (m Model) stepTalks(s *State, f Frame, actors []Actor, byID map[string]Actor, elapsed, day int64) []Speech {
	if len(m.Exchanges) == 0 {
		return nil
	}
	var started []Speech
	keep := s.Talks[:0]
	for _, run := range s.Talks {
		e, ok := m.Exchange(run.Exchange)
		if !ok || run.Turn >= len(e.Turns) {
			continue
		}
		a, okA := byID[run.A]
		b, okB := byID[run.B]
		if !okA || !okB || !m.talkersFit(*e, a, b) {
			continue // one of them left: the answer is lost, as with a pack's reply
		}
		turn := e.Turns[run.Turn]
		speaker, to := a, b
		if turn.By == 1 {
			speaker, to = b, a
		}
		if !m.hears(f.Hero, speaker, e.HeroWithinCells) {
			continue
		}
		if f.Tick < run.Due {
			keep = append(keep, run)
			continue
		}
		if s.speaking(speaker.ID, f.Tick) {
			if f.Tick < run.GiveUp {
				keep = append(keep, run)
			}
			continue
		}
		sp := m.sayTurn(s, *e, turn, speaker.ID, to.ID, f)
		started = append(started, sp)
		run.Turn++
		if run.Turn < len(e.Turns) {
			run.Due = sp.Until + m.ticks(m.Talk.PauseMS)
			run.GiveUp = run.Due + m.ticks(m.Talk.GiveUpMS)
			keep = append(keep, run)
		}
	}
	s.Talks = keep
	if len(s.Talks) == 0 {
		s.Talks = nil
	}
	if f.Hero.Level == "" || s.audible(f.Tick) >= m.MaxActive || len(s.Talks) > 0 {
		return started
	}
	hour := f.Now / 3600
	if s.Talk.Hour != hour {
		s.Talk.Hour, s.Talk.Count = hour, 0
	}
	if (s.Talk.Last > 0 && f.Now < s.Talk.Last+m.Talk.GapSeconds) || s.Talk.Count >= m.Talk.MaxPerHour {
		return started
	}
	if chance := elapsed * 1000000 / m.Talk.MeanWaitSeconds; chance < 1000000 && int64(Roll(f.Seed, "barks.talk", f.Hero.Level, f.Tick)%1000000) >= chance {
		return started
	}
	type cand struct {
		e    *Exchange
		a, b Actor
	}
	var cands []cand
	total := 0
	for i := range m.Exchanges {
		e := &m.Exchanges[i]
		if last, said := s.Lines[e.ID]; said && f.Now < last+e.CooldownSeconds {
			continue
		}
		inPeriod := false
		for _, pr := range e.Periods {
			inPeriod = inPeriod || pr.Has(day)
		}
		if !inPeriod {
			continue
		}
		casteOf := func(o Actor) string {
			if o.Caste != "" {
				return o.Caste
			}
			return m.Castes[o.ID]
		}
		free := func(o Actor) bool { return o.ID != f.Hero.ID && !s.speaking(o.ID, f.Tick) }
		var a, b Actor
		found := false
		for _, first := range actors {
			if first.ID != e.A.Who && (e.A.Who != Anyone || casteOf(first) != e.A.Caste) {
				continue
			}
			if !free(first) || !m.hears(f.Hero, first, e.HeroWithinCells) || !postureFits(e.A, first) {
				continue
			}
			best := -1
			for _, o := range actors {
				if o.ID == first.ID || !free(o) || !m.talkersFit(*e, first, o) {
					continue
				}
				if o.ID != e.B.Who && (e.B.Who != Anyone || casteOf(o) != e.B.Caste) {
					continue
				}
				if d := dist2(first.At, o.At); best < 0 || d < best {
					best, b, found = d, o, true
				}
			}
			if found {
				a = first
				break
			}
		}
		if !found {
			continue
		}
		cands = append(cands, cand{e, a, b})
		total += e.Weight
	}
	if total == 0 {
		return started
	}
	pick := int(Roll(f.Seed, "barks.talk.pick", f.Hero.Level, f.Tick) % uint64(total))
	var chosen cand
	for _, c := range cands {
		if pick < c.e.Weight {
			chosen = c
			break
		}
		pick -= c.e.Weight
	}
	sp := m.sayTurn(s, *chosen.e, chosen.e.Turns[0], chosen.a.ID, chosen.b.ID, f)
	started = append(started, sp)
	s.Lines[chosen.e.ID] = f.Now
	s.Talk.Last = f.Now
	s.Talk.Count++
	due := sp.Until + m.ticks(m.Talk.PauseMS)
	s.Talks = append(s.Talks, TalkRun{Exchange: chosen.e.ID, A: chosen.a.ID, B: chosen.b.ID, Turn: 1, Due: due, GiveUp: due + m.ticks(m.Talk.GiveUpMS)})
	return started
}

func (m Model) sayTurn(s *State, e Exchange, t Turn, speaker, to string, f Frame) Speech {
	s.Serial++
	caste := e.A.Caste
	if t.By == 1 {
		caste = e.B.Caste
	}
	face, _ := m.Face(caste)
	sp := Speech{Serial: s.Serial, Kind: "talk", Line: t.ID, Key: t.ID, Speaker: speaker, Caste: caste, To: to, Gesture: t.Gesture, Second: f.Now}
	m.clock(&sp, face, t.Measure, nil, f.Tick)
	s.Active = append(s.Active, sp)
	return sp
}

func (s *State) speaking(id string, tick int64) bool {
	for _, a := range s.Active {
		if a.Speaker == id && a.Kind != "thought" && tick < a.Until {
			return true
		}
	}
	return false
}
func (s *State) audible(tick int64) int {
	n := 0
	for _, a := range s.Active {
		if a.Kind != "thought" && tick < a.Until {
			n++
		}
	}
	return n
}

func dist2(a, b Cell) int { dx, dy := a.X-b.X, a.Y-b.Y; return dx*dx + dy*dy }

// hears: doc 20 — the hero hears a voice of his level within the pack's radius (cells).
func (m Model) hears(hero, speaker Actor, cells int) bool {
	return hero.Level != "" && hero.Level == speaker.Level && dist2(hero.At, speaker.At) <= cells*cells
}
func (m Model) inPeriod(p Pack, when []string, day int64) bool {
	for _, w := range when {
		for _, pr := range p.Periods {
			if pr.ID == w && pr.Has(day) {
				return true
			}
		}
	}
	return false
}

// speakersOf: the named speaker when he stands at the place, or anyone of the line's caste who does.
func (m Model) speakersOf(l Line, place Place, actors []Actor, hero string) []Actor {
	var out []Actor
	for _, a := range actors {
		if a.ID == hero || !place.Has(a.Level, a.At) || m.placeOf(a) != place.ID {
			continue
		}
		caste := a.Caste
		if caste == "" {
			caste = m.Castes[a.ID]
		}
		if l.Speaker == a.ID || (l.Speaker == Anyone && caste == l.Caste) {
			out = append(out, a)
		}
	}
	return out
}

// placeOf: the most specific place that holds the actor (the stool over the hall around it).
func (m Model) placeOf(a Actor) string {
	best, area := "", -1
	for _, p := range m.Places {
		if !p.Has(a.Level, a.At) {
			continue
		}
		if n := p.Area(); area < 0 || n < area {
			best, area = p.ID, n
		}
	}
	return best
}

func (m Model) requires(l Line, a Actor, byID map[string]Actor, f Frame, s *State) bool {
	r := l.Requires
	if r.SpeakerSeated && !a.Seated {
		return false
	}
	if r.SpeakerOnPost && !a.OnPost {
		return false
	}
	if r.TargetOnPost || r.TargetWithinCells > 0 {
		target := r.Target
		if target == "" {
			target = l.To
		}
		t, ok := byID[target]
		if !ok || (r.TargetOnPost && !t.OnPost) || (r.TargetWithinCells > 0 && (t.Level != a.Level || dist2(t.At, a.At) > r.TargetWithinCells*r.TargetWithinCells)) {
			return false
		}
	}
	if r.WalkersWithinCells > 0 {
		near := false
		for _, o := range f.Actors {
			near = near || (o.ID != a.ID && o.Moving && o.Level == a.Level && dist2(o.At, a.At) <= r.WalkersWithinCells*r.WalkersWithinCells)
		}
		if !near {
			return false
		}
	}
	if r.HeroPassingCells > 0 {
		if f.Tick-s.Hero.Moved > m.MovingTicks || s.Hero.Moved == 0 || f.Hero.Level != a.Level || dist2(f.Hero.At, a.At) > r.HeroPassingCells*r.HeroPassingCells {
			return false
		}
	}
	return true
}

// reacts: every react of the line holds (a stub never confirms anything).
func (m Model) reacts(s *State, l Line, place Place, speaker string, f Frame) bool {
	for _, r := range l.React {
		ev, ok := m.Event(r.Event)
		if !ok || ev.Source == "stub" {
			return false
		}
		holds, fresh := false, false
		for _, fact := range f.Facts {
			if fact.Event != r.Event || (r.Subject != "" && fact.Subject != r.Subject) {
				continue
			}
			switch {
			case r.Subject != "":
				// pinned to its subject whatever the scope
			case ev.Scope == "world":
				if fact.Level != "" && fact.Level != place.Level {
					continue
				}
			case ev.Scope == "place":
				match := false
				for _, sub := range place.Subjects {
					match = match || sub == fact.Subject
				}
				if !match {
					continue
				}
			case ev.Scope == "speaker":
				if fact.Subject != speaker {
					continue
				}
			}
			holds = true
			if f.Now-s.Facts[factKey(fact)] <= r.WithinSeconds {
				fresh = true
			}
		}
		if r.Negate == holds || (!r.Negate && !fresh) {
			return false
		}
	}
	return true
}

func (m Model) ticks(ms int64) int64 { return (ms*m.TickHz + 999) / 1000 }

// Duration of a text in one locale under a face (doc 94 §3): the reveal, the hold to read it, the fade.
func (m Model) reveal(face Face, ms Measure) int64 {
	t := m.Timing
	cps := int64(face.CPS)
	switch face.Reveal {
	case "set":
		return t.SetMS
	case "stamped":
		words := int64(ms.Words)
		if words < 1 {
			words = 1
		}
		per := int64(ms.Runes) * 1000 / cps / words
		if per < t.StampMinMS {
			per = t.StampMinMS
		}
		return per*words + int64(ms.Stops)*t.StopMS/2
	default:
		return int64(ms.Runes)*1000/cps + int64(ms.Commas)*t.CommaMS + int64(ms.Stops)*t.StopMS
	}
}
func (m Model) hold(ms Measure) int64 {
	h := m.Timing.HoldBaseMS + int64(ms.Words)*60000/m.Timing.WordsPerMinute
	if h < m.Timing.HoldMinMS {
		h = m.Timing.HoldMinMS
	}
	return h
}

// clock: the longest locale decides, so the line lives long enough in every language.
func (m Model) clock(sp *Speech, face Face, measure map[string]Measure, fix map[string]Fix, start int64) {
	for _, loc := range m.Locales {
		ms := measure[loc]
		if r := m.reveal(face, ms); r > sp.RevealMS {
			sp.RevealMS = r
		}
		if h := m.hold(ms); h > sp.HoldMS {
			sp.HoldMS = h
		}
		if fx, ok := fix[loc]; ok {
			if v := m.Timing.FixPauseMS + m.Timing.StrikeMS + m.reveal(face, fx.Measure); v > sp.FixMS {
				sp.FixMS = v
			}
		}
	}
	sp.FadeMS = m.Timing.FadeMS
	sp.Start = start
	sp.Until = start + m.ticks(sp.RevealMS+sp.FixMS+sp.HoldMS+sp.FadeMS)
}

func (m Model) say(s *State, l Line, speaker, place string, f Frame) Speech {
	s.Serial++
	face, _ := m.Face(l.Caste)
	sp := Speech{Serial: s.Serial, Kind: "line", Line: l.ID, Key: l.ID, Speaker: speaker, Caste: l.Caste, To: l.To, Place: place, Sfx: l.Sfx, Second: f.Now}
	m.clock(&sp, face, l.Measure, l.Fix, f.Tick)
	s.Active = append(s.Active, sp)
	return sp
}
func (m Model) reply(s *State, r Reply, p Pending, f Frame) Speech {
	s.Serial++
	sp := Speech{Serial: s.Serial, Kind: "reply", Line: r.ID, Key: r.Key, Speaker: r.Speaker, Caste: r.Caste, Place: p.Place, Second: f.Now}
	if r.Kind == "gesture" {
		sp.Kind, sp.Key, sp.Gesture = "gesture", "", r.Gesture
		sp.Start, sp.HoldMS = f.Tick, m.Timing.GestureMS
		sp.FadeMS = m.Timing.FadeMS
		sp.Until = f.Tick + m.ticks(sp.HoldMS+sp.FadeMS)
	} else {
		face, _ := m.Face(r.Caste)
		m.clock(&sp, face, r.Measure, nil, f.Tick)
	}
	s.Active = append(s.Active, sp)
	return sp
}

// Roll is the counter random of the engine: the same seed, stream, key and counter give the same number on every run.
func Roll(seed int64, stream, key string, counter int64) uint64 {
	h := uint64(14695981039346656037)
	add := func(v uint64) {
		for i := 0; i < 8; i++ {
			h ^= v & 255
			h *= 1099511628211
			v >>= 8
		}
	}
	str := func(x string) {
		add(uint64(len(x)))
		for i := 0; i < len(x); i++ {
			h ^= uint64(x[i])
			h *= 1099511628211
		}
	}
	add(uint64(seed))
	str(stream)
	str(key)
	add(uint64(counter))
	h ^= h >> 30
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 27
	h *= 0x94d049bb133111eb
	return h ^ (h >> 31)
}

// MeasureText counts what the timing needs: runes, words (a lone dash is not a word), commas and stops.
func MeasureText(text string) Measure {
	ms := Measure{Runes: len([]rune(text))}
	for _, w := range strings.Fields(text) {
		if w != "—" && w != "–" && w != "-" {
			ms.Words++
		}
	}
	for _, r := range text {
		switch r {
		case ',', ';', ':', '—':
			ms.Commas++
		case '.', '!', '?':
			ms.Stops++
		}
	}
	return ms
}
