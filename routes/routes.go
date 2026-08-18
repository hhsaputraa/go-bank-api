package routes

import (
	"net/http"
	"net/http/pprof"
	"time"

	auth "go-bank-api/auth"
	config "go-bank-api/config"
	controllers "go-bank-api/controllers"
	"go-bank-api/middleware"
	users "go-bank-api/modules/users"
)

func RegisterRoutes(mux *http.ServeMux) {
	// Rate Limiters
	authRateLimiter := middleware.RateLimitMiddleware(10, 1*time.Minute)
	queryRateLimiter := middleware.RateLimitMiddleware(30, 1*time.Minute)
	adminRateLimiter := middleware.RateLimitMiddleware(10, 1*time.Minute)

	// Public Routes
	mux.HandleFunc("/health", controllers.HandleHealthCheck)
	mux.Handle("/api/auth/register", authRateLimiter(http.HandlerFunc(controllers.HandleRegister)))
	mux.Handle("/api/auth/login", authRateLimiter(http.HandlerFunc(controllers.HandleLogin)))
	mux.Handle("/api/auth/login-otp", authRateLimiter(http.HandlerFunc(controllers.HandleLoginOTP)))
	mux.Handle("/api/auth/logout", auth.AuthMiddleware(controllers.HandleLogout))
	mux.Handle("/api/auth/me", auth.AuthMiddleware(controllers.HandleMe))
	mux.Handle("/api/auth/change-password", auth.AuthMiddleware(controllers.HandleChangePassword))

	// Protected Routes (Harus Login / Pakai Token + Rate Limit)
	mux.Handle("/api/query", queryRateLimiter(auth.AuthMiddleware(controllers.HandleDynamicQuery)))
	mux.Handle("/api/enhance", queryRateLimiter(auth.AuthMiddleware(controllers.HandleEnhancePrompt)))
	mux.Handle("/api/upload-session", queryRateLimiter(auth.AuthMiddleware(controllers.HandleUploadSession)))
	mux.Handle("/api/chat-session", queryRateLimiter(auth.AuthMiddleware(controllers.HandleChatSession)))

	// Feedback Routes (Protected)
	mux.Handle("/api/feedback/koreksi", auth.AuthMiddleware(controllers.HandleFeedbackKoreksi))

	// Admin Routes (Protected: Harus Login + Role Admin + Rate Limit)
	statusRateLimiter := middleware.RateLimitMiddleware(120, 1*time.Minute)

	mux.Handle("/admin/retrain", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminRetrain)))
	mux.Handle("/admin/retrain/status", statusRateLimiter(auth.AdminMiddleware(controllers.HandleAdminRetrainStatus)))
	mux.Handle("/admin/otp/generate", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminGenerateOTP)))
	mux.Handle("/admin/users", adminRateLimiter(auth.AdminMiddleware(users.HandleGetAllUsersOracle)))
	mux.Handle("/admin/qdrant/list", statusRateLimiter(auth.AdminMiddleware(controllers.HandleAdminListQdrant)))
	mux.Handle("/admin/qdrant/delete", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminDeleteQdrant)))
	mux.Handle("/admin/cache/create", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminCacheCreate)))
	mux.Handle("/admin/qdrant/update", adminRateLimiter(auth.AdminMiddleware(controllers.HandleAdminQdrantUpdate)))

	// Observability & Profiling (pprof) - Accessible in development or by admin
	registerPprofRoutes(mux)
}

func registerPprofRoutes(mux *http.ServeMux) {
	// In production, guard pprof behind AdminMiddleware; in development, allow direct access
	wrapPprof := func(h http.HandlerFunc) http.Handler {
		if config.AppConfig != nil && config.AppConfig.AppEnv == "production" {
			return auth.AdminMiddleware(h)
		}
		return h
	}

	mux.Handle("/debug/pprof/", wrapPprof(pprof.Index))
	mux.Handle("/debug/pprof/cmdline", wrapPprof(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", wrapPprof(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", wrapPprof(pprof.Symbol))
	mux.Handle("/debug/pprof/trace", wrapPprof(pprof.Trace))
}
