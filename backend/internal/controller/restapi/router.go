package restapi

import (
	"net/http"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi/errmap"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/go-chi/chi/v5"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
	httpmiddleware "github.com/wahrwelt-kit/go-httpkit/httputil/middleware"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func NewRouter(
	video VideoUsecase,
	checkers map[string]httpkit.Checker,
	healthTimeout time.Duration,
	logger logkit.Logger,
) http.Handler {
	router := chi.NewRouter()
	router.Use(httpmiddleware.RequestID())
	router.Use(httpmiddleware.Logger(logger, nil))

	handler := NewHandler(
		video,
		WithHealthCheckers(checkers, healthTimeout),
	)
	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: errmap.WriteOpenAPI,
	}

	router.Get("/healthz", handler.GetHealthz)
	router.Route("/api", func(api chi.Router) {
		api.Get("/videos", wrapper.ListVideos)
		api.Post("/videos/{id}/view", wrapper.IncrementVideoViews)
	})

	return router
}
