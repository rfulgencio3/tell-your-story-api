package service

import (
	"testing"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
)

func TestCalculateThreeLiesScoreDelta(t *testing.T) {
	t.Parallel()

	truthSet := domain.TruthSet{
		ID:                 "truth-set-1",
		AuthorUserID:       "author",
		TrueStatementIndex: 2,
	}

	testCases := []struct {
		name            string
		votes           []domain.TruthSetVote
		wantAuthor      int
		wantPlayerScore map[string]int
	}{
		{
			name: "unique correct vote gives 12 points and author gets zero",
			votes: []domain.TruthSetVote{
				{UserID: "guest-1", SelectedStatementIndex: 2},
			},
			wantAuthor: 0,
			wantPlayerScore: map[string]int{
				"guest-1": 12,
			},
		},
		{
			name: "shared correct votes give 10 points each and author gets zero",
			votes: []domain.TruthSetVote{
				{UserID: "guest-1", SelectedStatementIndex: 2},
				{UserID: "guest-2", SelectedStatementIndex: 2},
			},
			wantAuthor: 0,
			wantPlayerScore: map[string]int{
				"guest-1": 10,
				"guest-2": 10,
			},
		},
		{
			name: "majority wrong gives author five points",
			votes: []domain.TruthSetVote{
				{UserID: "guest-1", SelectedStatementIndex: 1},
				{UserID: "guest-2", SelectedStatementIndex: 1},
				{UserID: "guest-3", SelectedStatementIndex: 2},
			},
			wantAuthor: 5,
			wantPlayerScore: map[string]int{
				"guest-3": 12,
			},
		},
		{
			name: "tie between correct and wrong gives author two points",
			votes: []domain.TruthSetVote{
				{UserID: "guest-1", SelectedStatementIndex: 2},
				{UserID: "guest-2", SelectedStatementIndex: 1},
			},
			wantAuthor: 2,
			wantPlayerScore: map[string]int{
				"guest-1": 12,
			},
		},
		{
			name: "nobody correct gives author ten points",
			votes: []domain.TruthSetVote{
				{UserID: "guest-1", SelectedStatementIndex: 1},
				{UserID: "guest-2", SelectedStatementIndex: 4},
			},
			wantAuthor:      10,
			wantPlayerScore: map[string]int{},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotPlayerScore, gotAuthor, revealedVotes := calculateThreeLiesScoreDelta(truthSet, testCase.votes)
			if gotAuthor != testCase.wantAuthor {
				t.Fatalf("author points = %d, want %d", gotAuthor, testCase.wantAuthor)
			}
			if len(gotPlayerScore) != len(testCase.wantPlayerScore) {
				t.Fatalf("player score count = %d, want %d", len(gotPlayerScore), len(testCase.wantPlayerScore))
			}
			for userID, wantPoints := range testCase.wantPlayerScore {
				if gotPlayerScore[userID] != wantPoints {
					t.Fatalf("player %q points = %d, want %d", userID, gotPlayerScore[userID], wantPoints)
				}
			}
			if len(revealedVotes) != len(testCase.votes) {
				t.Fatalf("revealed votes count = %d, want %d", len(revealedVotes), len(testCase.votes))
			}
		})
	}
}

func TestBuildThreeLiesFinalRankingSharesTiePositions(t *testing.T) {
	t.Parallel()

	users := []domain.User{
		{ID: "j1", Nickname: "J1"},
		{ID: "j2", Nickname: "J2"},
		{ID: "j3", Nickname: "J3"},
		{ID: "j4", Nickname: "J4"},
	}
	scores := []domain.RoomScore{
		{RoomID: "room-1", UserID: "j1", Score: 32, UpdatedAt: time.Now().UTC()},
		{RoomID: "room-1", UserID: "j2", Score: 18, UpdatedAt: time.Now().UTC()},
		{RoomID: "room-1", UserID: "j3", Score: 32, UpdatedAt: time.Now().UTC()},
		{RoomID: "room-1", UserID: "j4", Score: 20, UpdatedAt: time.Now().UTC()},
	}

	ranking := buildThreeLiesFinalRanking(users, scores)
	if len(ranking) != 4 {
		t.Fatalf("ranking count = %d, want 4", len(ranking))
	}

	if ranking[0].Position != 1 || ranking[1].Position != 1 || ranking[2].Position != 3 || ranking[3].Position != 4 {
		t.Fatalf("ranking positions = %v, want [1 1 3 4]", []int{ranking[0].Position, ranking[1].Position, ranking[2].Position, ranking[3].Position})
	}
}
