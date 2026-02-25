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
	Entries     []Entry
	RankData    RankDataFile
	OrganizerID string // 発案者の Discord UserID
	MessageID   string // Discord メッセージID（Embed 更新用）
	ChannelID   string // チャンネルID
	IsOpen      bool   // 募集中かどうか
}

func NewRecruitment(rankData RankDataFile) *Recruitment {
	return &Recruitment{Entries: []Entry{}, RankData: rankData}
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
			return true
		}
	}
	return false
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
	if len(players) < 10 {
		return nil
	}
	target := len(players) / 5 * 5

	shuffled := append([]ScoredPlayer(nil), players...)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	shuffled = shuffled[:target]

	sort.Slice(shuffled, func(i, j int) bool { return shuffled[i].Score > shuffled[j].Score })
	return r.balancedScoreTeams(shuffled)
}

func (r *Recruitment) balancedScoreTeams(players []ScoredPlayer) [][]ScoredPlayer {
	teamCount := len(players) / 5
	var best [][]ScoredPlayer
	bestVariance := math.MaxFloat64

	for range 20 {
		p := append([]ScoredPlayer(nil), players...)
		highEnd := len(p) / 3
		midEnd := highEnd * 2
		high := append([]ScoredPlayer(nil), p[:highEnd]...)
		mid := append([]ScoredPlayer(nil), p[highEnd:midEnd]...)
		low := append([]ScoredPlayer(nil), p[midEnd:]...)

		rand.Shuffle(len(high), func(i, j int) { high[i], high[j] = high[j], high[i] })
		rand.Shuffle(len(mid), func(i, j int) { mid[i], mid[j] = mid[j], mid[i] })
		rand.Shuffle(len(low), func(i, j int) { low[i], low[j] = low[j], low[i] })

		teams := make([][]ScoredPlayer, teamCount)
		for i := 0; i < teamCount; i++ {
			highCount := min(rand.Intn(2)+1, len(high))
			for range highCount {
				teams[i] = append(teams[i], high[len(high)-1])
				high = high[:len(high)-1]
			}
			midCount := min(2, len(mid))
			for range midCount {
				teams[i] = append(teams[i], mid[len(mid)-1])
				mid = mid[:len(mid)-1]
			}
			for len(teams[i]) < 5 && len(low) > 0 {
				teams[i] = append(teams[i], low[len(low)-1])
				low = low[:len(low)-1]
			}
		}

		remaining := append(append(high, mid...), low...)
		for _, pl := range remaining {
			for i := range teams {
				if len(teams[i]) < 5 {
					teams[i] = append(teams[i], pl)
					break
				}
			}
		}

		variance := teamScoreVariance(teams)
		if variance < bestVariance {
			bestVariance = variance
			best = teams
		}
	}

	return best
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
