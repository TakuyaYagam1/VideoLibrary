package mock

import (
	"testing"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
)

func TestMocksImplementUsecasePorts(t *testing.T) {
	var _ usecase.VideoRepository = (*MockVideoRepository)(nil)
	var _ usecase.VideoCache = (*MockVideoCache)(nil)
}
