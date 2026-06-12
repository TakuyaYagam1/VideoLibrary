package v1

import (
	"net/http"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi/middleware"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/wahrwelt-kit/go-httpkit/httperr"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
	httpmiddleware "github.com/wahrwelt-kit/go-httpkit/httputil/middleware"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func NewRouter(
	video usecase.VideoUsecase,
	checkers map[string]httpkit.Checker,
	healthTimeout time.Duration,
	allowedOrigins []string,
	logger logkit.Logger,
) http.Handler {
	router := chi.NewRouter()
	router.Use(httpmiddleware.Recoverer(logger))
	router.Use(httpmiddleware.RequestID())
	if len(allowedOrigins) > 0 {
		router.Use(cors.Handler(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
			ExposedHeaders:   []string{"X-Request-ID"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}
	router.Use(httpmiddleware.SecurityHeaders(false))
	router.Use(httpmiddleware.Logger(logger, nil))

	handler := NewHandler(
		video,
		WithHealthCheckers(checkers, healthTimeout),
	)
	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: writeOpenAPIError,
	}

	router.Get("/livez", handler.GetLivez)
	router.Get("/readyz", handler.GetHealthz)
	router.Get("/healthz", handler.GetHealthz)
	incrementLimiter := middleware.NewRateLimiter(60, time.Minute)
	router.Route("/api", func(api chi.Router) {
		api.Get("/videos", wrapper.ListVideos)
		api.Get("/videos/{id}", wrapper.GetVideo)
		api.With(incrementLimiter.Middleware).Post("/videos/{id}/view", wrapper.IncrementVideoViews)
	})

	return router
}

func writeOpenAPIError(w http.ResponseWriter, r *http.Request, err error) {
	httpkit.HandleError(w, r, httperr.New(err, http.StatusBadRequest, httperr.CodeInvalidID))
}
