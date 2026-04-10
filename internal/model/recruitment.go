package model

import (
	"math"
	"math/rand"
	"sort"
)

type Entry struct {
	UserID string
	Name   string
}

type ScoredPlayer struct {
	ID    string
	Name  string
	Score float64
}

type Recruitment struct {
	Entries          []Entry
	Parties          map[string][]string // hostUserID -> member userIDs (hostを除く)
	RankData         RankDataFile
	OrganizerID      string // 発案者の Discord UserID
	MessageID        string // Discord メッセージID（Embed 更新用）
	ChannelID        string // チャンネルID
	GuildID          string // サーバーID
	IsOpen           bool   // 募集中かどうか
	HasAssigned      bool   // 振り分け結果を一度でも送信したか
	AssignInProgress bool   // 振り分け処理中かどうか
}

func NewRecruitment(rankData RankDataFile) *Recruitment {
	return &Recruitment{Entries: []Entry{}, Parties: make(map[string][]string), RankData: rankData}
}

func (r *Recruitment) AddEntry(userID, name string) bool {
	for _, e := range r.Entries {
		if e.UserID == userID {
			return false
		}
	}
	r.Entries = append(r.Entries, Entry{UserID: userID, Name: name})
	return true
}

// RemoveEntry はエントリーを取り消す。成功したら true、存在しない場合は false を返す。
func (r *Recruitment) RemoveEntry(userID string) bool {
	for i, e := range r.Entries {
		if e.UserID == userID {
			r.Entries = append(r.Entries[:i], r.Entries[i+1:]...)
			return r.removeEntryWithPartyCascade(userID)
		}
	}
	return false
}

func (r *Recruitment) removeEntryWithPartyCascade(userID string) bool {
	hostID, members, found := r.findPartyByMember(userID)
	if !found {
		return true
	}

	targetIDs := append([]string{hostID}, members...)
	for _, id := range targetIDs {
		if id == userID {
			continue
		}
		for i := 0; i < len(r.Entries); i++ {
			if r.Entries[i].UserID == id {
				r.Entries = append(r.Entries[:i], r.Entries[i+1:]...)
				i--
			}
		}
	}
	delete(r.Parties, hostID)
	return true
}

func (r *Recruitment) SetParty(hostID string, members []string) {
	if r.Parties == nil {
		r.Parties = make(map[string][]string)
	}
	cleaned := make([]string, 0, len(members))
	seen := map[string]struct{}{}
	for _, id := range members {
		if id == "" || id == hostID {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		delete(r.Parties, hostID)
		return
	}
	r.Parties[hostID] = cleaned
}

func (r *Recruitment) ClearParty(hostID string) {
	delete(r.Parties, hostID)
}

func (r *Recruitment) IsEntered(userID string) bool {
	for _, e := range r.Entries {
		if e.UserID == userID {
			return true
		}
	}
	return false
}

func (r *Recruitment) findPartyByMember(userID string) (string, []string, bool) {
	for hostID, members := range r.Parties {
		if hostID == userID {
			return hostID, append([]string{}, members...), true
		}
		for _, member := range members {
			if member == userID {
				return hostID, append([]string{}, members...), true
			}
		}
	}
	return "", nil, false
}

func (r *Recruitment) FindPartyHostOf(userID string) (string, bool) {
	hostID, _, found := r.findPartyByMember(userID)
	return hostID, found
}

func (r *Recruitment) CalculatePlayerScore(highestRank Rank) float64 {
	if highestRank.Rank == "top500" {
		return 4500
	}
	divisions, ok := r.RankData.Ranks[highestRank.Rank]
	if !ok {
		return 0
	}
	score, ok := divisions[highestRank.Division]
	if !ok {
		return 0
	}
	return score
}

func (r *Recruitment) MakeTeams(players []ScoredPlayer) [][]ScoredPlayer {
	teams, _ := r.MakeTeamsWithRemainder(players)
	return teams
}

func (r *Recruitment) MakeTeamsWithRemainder(players []ScoredPlayer) ([][]ScoredPlayer, []ScoredPlayer) {
	if len(players) < 10 {
		return nil, nil
	}
	groups := r.buildPartyGroups(players)
	selected, remainder := selectGroupedPlayers(groups)
	if len(selected) < 10 {
		return nil, nil
	}

	sort.Slice(selected, func(i, j int) bool { return selected[i].Score > selected[j].Score })
	teams := r.balancedScoreTeams(selected)
	if teams == nil {
		return nil, nil
	}
	return teams, remainder
}

func (r *Recruitment) balancedScoreTeams(players []ScoredPlayer) [][]ScoredPlayer {
	grouped := r.buildPartyGroups(players)
	if len(grouped) == 0 {
		return nil
	}
	teamCount := len(players) / 5
	if !canPackPartyGroups(grouped, teamCount) {
		return nil
	}

	var best [][]ScoredPlayer
	bestVariance := math.MaxFloat64
	for range 200 {
		teams, ok := buildPackedTeamsTrial(grouped, teamCount)
		if !ok {
			continue
		}
		variance := teamScoreVariance(teams)
		if variance < bestVariance {
			bestVariance = variance
			best = teams
		}
	}
	return best
}

type playerGroup struct {
	Players []ScoredPlayer
}

func (r *Recruitment) buildPartyGroups(players []ScoredPlayer) []playerGroup {
	playerByID := make(map[string]ScoredPlayer, len(players))
	for _, p := range players {
		playerByID[p.ID] = p
	}
	used := make(map[string]bool, len(players))
	groups := make([]playerGroup, 0, len(players))
	for hostID, members := range r.Parties {
		if used[hostID] {
			continue
		}
		host, ok := playerByID[hostID]
		if !ok {
			continue
		}
		groupPlayers := []ScoredPlayer{host}
		used[hostID] = true
		valid := true
		for _, memberID := range members {
			member, ok := playerByID[memberID]
			if !ok {
				valid = false
				break
			}
			groupPlayers = append(groupPlayers, member)
		}
		if !valid || len(groupPlayers) > 5 {
			continue
		}
		for _, p := range groupPlayers[1:] {
			used[p.ID] = true
		}
		groups = append(groups, playerGroup{Players: groupPlayers})
	}

	for _, p := range players {
		if used[p.ID] {
			continue
		}
		groups = append(groups, playerGroup{Players: []ScoredPlayer{p}})
	}
	return groups
}

func selectGroupedPlayers(groups []playerGroup) ([]ScoredPlayer, []ScoredPlayer) {
	if len(groups) == 0 {
		return nil, nil
	}
	bestSelection := make([]bool, len(groups))
	bestTotal := -1
	groupCount := len(groups)
	if groupCount <= 20 {
		limit := 1 << groupCount
		for mask := 0; mask < limit; mask++ {
			selection := make([]bool, len(groups))
			total := 0
			for idx := range groups {
				if mask&(1<<idx) == 0 {
					continue
				}
				selection[idx] = true
				total += len(groups[idx].Players)
			}
			if total < 10 || total%5 != 0 {
				continue
			}
			if total > bestTotal {
				bestTotal = total
				bestSelection = selection
			}
		}
	} else {
		for range 6000 {
			selection := make([]bool, len(groups))
			total := 0
			for idx := range groups {
				if rand.Intn(2) == 0 {
					continue
				}
				selection[idx] = true
				total += len(groups[idx].Players)
			}
			if total < 10 || total%5 != 0 {
				continue
			}
			if total > bestTotal {
				bestTotal = total
				bestSelection = selection
			}
		}
	}

	if bestTotal < 10 {
		return nil, nil
	}

	selected := make([]ScoredPlayer, 0, bestTotal)
	remainder := make([]ScoredPlayer, 0)
	for idx, group := range groups {
		if !bestSelection[idx] {
			remainder = append(remainder, group.Players...)
			continue
		}
		selected = append(selected, group.Players...)
	}
	return selected, remainder
}

func canPackPartyGroups(groups []playerGroup, teamCount int) bool {
	sizes := make([]int, len(groups))
	for i, g := range groups {
		sizes[i] = len(g.Players)
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i] > sizes[j] })
	caps := make([]int, teamCount)
	for i := range caps {
		caps[i] = 5
	}
	var dfs func(idx int) bool
	dfs = func(idx int) bool {
		if idx == len(sizes) {
			for _, cap := range caps {
				if cap != 0 {
					return false
				}
			}
			return true
		}
		seen := map[int]struct{}{}
		for i := range caps {
			if caps[i] < sizes[idx] {
				continue
			}
			if _, ok := seen[caps[i]]; ok {
				continue
			}
			seen[caps[i]] = struct{}{}
			caps[i] -= sizes[idx]
			if dfs(idx + 1) {
				return true
			}
			caps[i] += sizes[idx]
		}
		return false
	}
	return dfs(0)
}

func buildPackedTeamsTrial(groups []playerGroup, teamCount int) ([][]ScoredPlayer, bool) {
	teams := make([][]ScoredPlayer, teamCount)
	teamSizes := make([]int, teamCount)
	teamScores := make([]float64, teamCount)
	shuffled := append([]playerGroup(nil), groups...)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	sort.Slice(shuffled, func(i, j int) bool { return len(shuffled[i].Players) > len(shuffled[j].Players) })
	for _, group := range shuffled {
		size := len(group.Players)
		candidates := make([]int, 0, teamCount)
		for idx := range teams {
			if teamSizes[idx]+size <= 5 {
				candidates = append(candidates, idx)
			}
		}
		if len(candidates) == 0 {
			return nil, false
		}
		bestTeam := candidates[0]
		bestGap := math.MaxFloat64
		groupAvg := teamAverage(group.Players)
		for _, idx := range candidates {
			avg := teamScores[idx]
			if teamSizes[idx] > 0 {
				avg /= float64(teamSizes[idx])
			}
			gap := math.Abs(avg - groupAvg)
			if gap < bestGap {
				bestGap = gap
				bestTeam = idx
			}
		}
		teams[bestTeam] = append(teams[bestTeam], group.Players...)
		teamSizes[bestTeam] += size
		for _, p := range group.Players {
			teamScores[bestTeam] += p.Score
		}
	}
	for _, s := range teamSizes {
		if s != 5 {
			return nil, false
		}
	}
	return teams, true
}

func teamAverage(players []ScoredPlayer) float64 {
	if len(players) == 0 {
		return 0
	}
	total := 0.0
	for _, p := range players {
		total += p.Score
	}
	return total / float64(len(players))
}

func teamScoreVariance(teams [][]ScoredPlayer) float64 {
	avgs := make([]float64, 0, len(teams))
	for _, t := range teams {
		if len(t) == 0 {
			continue
		}
		total := 0.0
		for _, p := range t {
			total += p.Score
		}
		avgs = append(avgs, total/float64(len(t)))
	}
	if len(avgs) == 0 {
		return math.MaxFloat64
	}
	mean := 0.0
	for _, v := range avgs {
		mean += v
	}
	mean /= float64(len(avgs))

	variance := 0.0
	for _, v := range avgs {
		d := v - mean
		variance += d * d
	}
	return variance / float64(len(avgs))
}
