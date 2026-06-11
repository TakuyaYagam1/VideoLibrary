package integration_test

import (
	"testing"

	repopostgres "github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
)

func TestVideoRepositoryImplementsUsecasePort(t *testing.T) {
	var _ usecase.VideoRepository = (*repopostgres.VideoRepository)(nil)
}
