package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

// Участники комнаты: список строится из истории (кто писал или представлялся), статус «на связи» вычисляется
// только по кнопке «Проверить статус» - автоматического опроса нет.

// Participant - участник комнаты.
type Participant struct {
	UID    string
	Name   string
	LastTS int64 // время последней активности (мс Unix)
	Self   bool
}

func (r *ChatRoom) hidePath() string {
	if r.path == "" {
		return ""
	}
	return strings.TrimSuffix(r.path, ".log") + ".hide"
}

// loadHidden: UID -> время (мс), когда участника убрали из списка. Вернётся в список, если напишет позже.
func (r *ChatRoom) loadHidden() map[string]int64 {
	h := map[string]int64{}
	if p := r.hidePath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &h)
		}
	}
	return h
}

// Roster возвращает участников комнаты (без убранных), свой UID - первым, остальные по имени.
func (r *ChatRoom) Roster(selfUID string) []Participant {
	hidden := r.loadHidden()
	r.mu.Lock()
	last := map[string]int64{}
	for _, m := range r.msgs {
		if m.UID != "" && m.TS > last[m.UID] {
			last[m.UID] = m.TS
		}
	}
	var out []Participant
	for uid, ts := range last {
		if uid != selfUID {
			if h, ok := hidden[uid]; ok && ts <= h {
				continue
			}
		}
		name := r.names[uid]
		if name == "" {
			name = "?"
		}
		out = append(out, Participant{UID: uid, Name: name, LastTS: ts, Self: uid == selfUID})
	}
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Self != out[j].Self {
			return out[i].Self
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// HideOffline убирает из списка всех, кто не на связи (keep(uid) == false), кроме себя. Возвращает число убранных.
func (r *ChatRoom) HideOffline(selfUID string, keep func(uid string) bool) int {
	hidden := r.loadHidden()
	now := nowMs()
	n := 0
	for _, p := range r.Roster(selfUID) {
		if p.Self || keep(p.UID) {
			continue
		}
		hidden[p.UID] = now
		n++
	}
	if hp := r.hidePath(); hp != "" && n > 0 {
		b, _ := json.Marshal(hidden)
		_ = os.WriteFile(hp, b, 0o600)
	}
	return n
}

// activeSince - UID, от которых были события не раньше since (мс).
func (r *ChatRoom) activeSince(since int64) map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]bool{}
	for _, m := range r.msgs {
		if m.UID != "" && m.TS >= since {
			out[m.UID] = true
		}
	}
	return out
}

// OnlineUIDs (локальный чат): у кого сейчас есть соединение и кто состоит в этой комнате.
func (n *ChatNode) OnlineUIDs(r *ChatRoom) map[string]bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := map[string]bool{n.UID: true}
	for _, c := range n.peers {
		if c.uid != "" && c.rooms[r.ID] {
			out[c.uid] = true
		}
	}
	return out
}

// CheckStatus (локальный чат): ищет устройства заново, даёт время на соединение.
func (n *ChatNode) CheckStatus(r *ChatRoom) {
	n.Scan(1500 * time.Millisecond)
	time.Sleep(1200 * time.Millisecond)
}

// presenceWindow - в интернет-чате «на связи» значит «была активность за последние 10 минут».
const presenceWindow = 10 * time.Minute

// OnlineUIDs (интернет-чат): по недавней активности.
func (n *NetChat) OnlineUIDs(r *ChatRoom) map[string]bool {
	out := r.activeSince(nowMs() - presenceWindow.Milliseconds())
	out[n.UserID()] = true
	return out
}

// CheckStatus (интернет-чат): объявляемся в комнате и немного ждём ответного «hi» от остальных.
func (n *NetChat) CheckStatus(r *ChatRoom) {
	_ = n.post(r, ChatMsg{Kind: "hi"})
	time.Sleep(3 * time.Second)
}
