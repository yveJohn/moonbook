package crawlboard

import "context"

type Board struct {
	ID, SourceID, SourceName, BoardName, BoardURL, BoardURLTemplate string
	LastCursor, FollowIntervalMinutes, FollowImportLimit, SortOrder string
	Remark, CreatedAt, UpdatedAt                                    string
	AutoFollowEnabled, Enabled                                      bool
}

type Input struct {
	SourceID              string `json:"sourceId"`
	BoardName             string `json:"boardName"`
	BoardURL              string `json:"boardUrl"`
	BoardURLTemplate      string `json:"boardUrlTemplate"`
	AutoFollowEnabled     bool   `json:"autoFollowEnabled"`
	FollowIntervalMinutes string `json:"followIntervalMinutes"`
	FollowImportLimit     string `json:"followImportLimit"`
	SortOrder             string `json:"sortOrder"`
	Enabled               bool   `json:"enabled"`
	Remark                string `json:"remark"`
}

type Repository interface {
	List(context.Context, string, string, string, int, int) ([]Board, int64, error)
	Create(context.Context, Input) (Board, error)
	Update(context.Context, int64, Input) (Board, error)
	Delete(context.Context, int64) error
}
