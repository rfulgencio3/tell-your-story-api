package service

import (
	"sort"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
)

const (
	threeLiesUniqueCorrectPoints = 12
	threeLiesSharedCorrectPoints = 10
	threeLiesMajorityWrongPoints = 5
	threeLiesTiePoints           = 2
	threeLiesNobodyCorrectPoints = 10
)

// ThreeLiesRevealedVote is the public reveal payload for one participant vote.
type ThreeLiesRevealedVote struct {
	UserID                 string `json:"user_id"`
	SelectedStatementIndex int    `json:"selected_statement_index"`
	IsCorrect              bool   `json:"is_correct"`
}

// ThreeLiesRevealState is the public reveal payload for the active truth set.
type ThreeLiesRevealState struct {
	TruthSet           domain.TruthSet         `json:"truth_set"`
	TrueStatementIndex int                     `json:"true_statement_index"`
	RevealedVotes      []ThreeLiesRevealedVote `json:"revealed_votes"`
}

// ThreeLiesVotingProgress contains aggregate progress for the active truth set.
type ThreeLiesVotingProgress struct {
	EligibleVoters int `json:"eligible_voters"`
	SubmittedVotes int `json:"submitted_votes"`
}

// ThreeLiesRankingEntry is the public final ranking entry for one participant.
type ThreeLiesRankingEntry struct {
	Position  int    `json:"position"`
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Score     int    `json:"score"`
}

func calculateThreeLiesScoreDelta(truthSet domain.TruthSet, votes []domain.TruthSetVote) (map[string]int, int, []ThreeLiesRevealedVote) {
	playerPoints := make(map[string]int, len(votes))
	revealedVotes := make([]ThreeLiesRevealedVote, 0, len(votes))

	correctVotes := 0
	for _, vote := range votes {
		isCorrect := vote.SelectedStatementIndex == truthSet.TrueStatementIndex
		if isCorrect {
			correctVotes++
		}
		revealedVotes = append(revealedVotes, ThreeLiesRevealedVote{
			UserID:                 vote.UserID,
			SelectedStatementIndex: vote.SelectedStatementIndex,
			IsCorrect:              isCorrect,
		})
	}

	sort.Slice(revealedVotes, func(i, j int) bool {
		return revealedVotes[i].UserID < revealedVotes[j].UserID
	})

	for _, revealedVote := range revealedVotes {
		if !revealedVote.IsCorrect {
			continue
		}
		if correctVotes == 1 {
			playerPoints[revealedVote.UserID] = threeLiesUniqueCorrectPoints
			continue
		}
		playerPoints[revealedVote.UserID] = threeLiesSharedCorrectPoints
	}

	totalVotes := len(votes)
	authorPoints := 0
	switch {
	case correctVotes == 0:
		authorPoints = threeLiesNobodyCorrectPoints
	case correctVotes == totalVotes:
		authorPoints = 0
	case correctVotes == totalVotes-correctVotes:
		authorPoints = threeLiesTiePoints
	case correctVotes < totalVotes-correctVotes:
		authorPoints = threeLiesMajorityWrongPoints
	default:
		authorPoints = 0
	}

	return playerPoints, authorPoints, revealedVotes
}

func buildThreeLiesFinalRanking(users []domain.User, scores []domain.RoomScore) []ThreeLiesRankingEntry {
	scoreByUserID := make(map[string]int, len(scores))
	for _, score := range scores {
		scoreByUserID[score.UserID] = score.Score
	}

	ranking := make([]ThreeLiesRankingEntry, 0, len(users))
	for _, user := range users {
		ranking = append(ranking, ThreeLiesRankingEntry{
			UserID:    user.ID,
			Nickname:  user.Nickname,
			AvatarURL: user.AvatarURL,
			Score:     scoreByUserID[user.ID],
		})
	}

	sort.Slice(ranking, func(i, j int) bool {
		if ranking[i].Score == ranking[j].Score {
			if ranking[i].Nickname == ranking[j].Nickname {
				return ranking[i].UserID < ranking[j].UserID
			}
			return ranking[i].Nickname < ranking[j].Nickname
		}
		return ranking[i].Score > ranking[j].Score
	})

	lastPosition := 0
	for index := range ranking {
		if index == 0 {
			lastPosition = 1
			ranking[index].Position = lastPosition
			continue
		}
		if ranking[index].Score == ranking[index-1].Score {
			ranking[index].Position = lastPosition
			continue
		}
		lastPosition = index + 1
		ranking[index].Position = lastPosition
	}

	return ranking
}

func cloneRevealVotes(votes []ThreeLiesRevealedVote) []ThreeLiesRevealedVote {
	return append([]ThreeLiesRevealedVote(nil), votes...)
}

func scoreUpdatedAt(now time.Time) time.Time {
	return now.UTC()
}
