package httperr

import (
	"errors"
	"net/http"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/httputil"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
)

const (
	CodeInvalidRequest = "invalid_request"
	CodeVideoNotFound  = "video_not_found"
	CodeInternal       = "internal_error"
)

func Write(w http.ResponseWriter, status int, code string, message string) {
	httputil.WriteJSON(w, status, openapi.ErrorResponse{
		Code:    code,
		Message: message,
	})
}

func WriteOpenAPI(w http.ResponseWriter, _ *http.Request, _ error) {
	Write(w, http.StatusBadRequest, CodeInvalidRequest, "invalid request")
}

func WriteUsecase(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrVideoNotFound) {
		Write(w, http.StatusNotFound, CodeVideoNotFound, domain.ErrVideoNotFound.Error())
		return
	}

	Write(w, http.StatusInternalServerError, CodeInternal, "internal server error")
}
