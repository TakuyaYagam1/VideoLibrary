package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
)

func TestErrVideoNotFoundIsSentinel(t *testing.T) {
	err := fmt.Errorf("load video: %w", domain.ErrVideoNotFound)

	if !errors.Is(err, domain.ErrVideoNotFound) {
		t.Fatalf("expected wrapped error to match ErrVideoNotFound")
	}
}
