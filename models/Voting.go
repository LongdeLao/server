package models

import "time"

// VotingEvent represents a voting event in the system
type VotingEvent struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Deadline    time.Time `json:"deadline"`
	Status      string    `json:"status"`
	OrganizerID int       `json:"organizer_id"`
	OrganizerName string  `json:"organizer_name,omitempty"`
	OrganizerRole string  `json:"organizer_role,omitempty"`
	VoteCount   int       `json:"vote_count,omitempty"`
	TotalVotes  int       `json:"total_votes,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	SubVotes    []SubVote `json:"sub_votes,omitempty"`
}

// SubVote represents a sub-vote within a voting event
type SubVote struct {
	ID          int          `json:"id"`
	EventID     int          `json:"event_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at,omitempty"`
	Options     []VoteOption `json:"options,omitempty"`
}

// VoteOption represents a voting option within a sub-vote
type VoteOption struct {
	ID            int       `json:"id"`
	SubVoteID     int       `json:"sub_vote_id"`
	Text          string    `json:"text"`
	HasCustomInput bool     `json:"has_custom_input"`
	VoteCount     int       `json:"vote_count,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

// UserVote represents a user's vote on a particular option
type UserVote struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	SubVoteID   int       `json:"sub_vote_id"`
	OptionID    int       `json:"option_id"`
	CustomInput string    `json:"custom_input,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

// UserVoteRequest is used for receiving vote submissions from clients
type UserVoteRequest struct {
	SubVoteID   int    `json:"sub_vote_id" binding:"required"`
	OptionID    int    `json:"option_id" binding:"required"`
	CustomInput string `json:"custom_input,omitempty"`
}

// VotingEventRequest is used for creating or updating a voting event
type VotingEventRequest struct {
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	Deadline    time.Time `json:"deadline" binding:"required"`
	Status      string    `json:"status" binding:"required"`
	SubVotes    []SubVoteRequest `json:"sub_votes" binding:"required"`
}

// SubVoteRequest is used for creating or updating a sub-vote
type SubVoteRequest struct {
	Title       string             `json:"title" binding:"required"`
	Description string             `json:"description"`
	Options     []VoteOptionRequest `json:"options" binding:"required"`
}

// VoteOptionRequest is used for creating or updating a vote option
type VoteOptionRequest struct {
	Text          string `json:"text" binding:"required"`
	HasCustomInput bool   `json:"has_custom_input"`
} 