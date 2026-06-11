package domain

import (
	"time"

	"github.com/google/uuid"
)

// Video describes a media file tracked by the library.
type Video struct {
	ID        uuid.UUID
	Title     string
	FilePath  string
	Views     int64
	CreatedAt time.Time
}
