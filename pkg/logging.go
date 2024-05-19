package pkg

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

func SetupLogger() {
	slog.SetDefault(
		slog.New(
			&slogHandlerWrapper{
				Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}),
				extractors: []Extractor{
					CorrelationIDExtractor,
				},
			},
		),
	)
}

func Err(err error) slog.Attr {
	return slog.String("err", err.Error())
}

type Extractor func(ctx context.Context) []slog.Attr

type slogHandlerWrapper struct {
	slog.Handler
	extractors []Extractor
}

func (s *slogHandlerWrapper) Handle(ctx context.Context, record slog.Record) error {
	for _, extractor := range s.extractors {
		record.AddAttrs(extractor(ctx)...)
	}

	return s.Handler.Handle(ctx, record)
}

func CorrelationIDExtractor(ctx context.Context) []slog.Attr {
	correlationID := GetCorrelationIDCtx(ctx)
	return []slog.Attr{slog.String("correlation_id", correlationID.String())}
}

func LoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		attrs := []any{
			slog.Group(
				"request",
				slog.String("method", r.Method),
				slog.String("url", r.URL.String()),
				slog.String("host", r.Host),
				slog.String("proto", r.Proto),
				slog.String("user_agent", r.UserAgent()),
			),
		}

		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				attrs = append(
					attrs,
					slog.Group("response",
						slog.Int("status_code", http.StatusInternalServerError),
						slog.Duration("duration", time.Since(start)),
					),
					slog.Group("panic",
						slog.Any("message", err),
						slog.String("stack", string(debug.Stack())),
					),
				)
				slog.ErrorContext(r.Context(), "server recovered from panic", attrs...)
			}
		}()

		flusher := w.(http.Flusher)
		hijacker := w.(http.Hijacker)
		mw := &metricsHttpWriter{
			ResponseWriter: w,
			Flusher:        flusher,
			Hijacker:       hijacker,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(mw, r)

		duration := time.Since(start)
		attrs = append(
			attrs,
			slog.Group(
				"response",
				slog.Int("status_code", mw.statusCode),
				slog.Duration("duration", duration),
			),
		)

		if mw.statusCode >= 500 {
			slog.ErrorContext(r.Context(), "request resulted with server error", attrs...)
		} else if mw.statusCode >= 400 {
			slog.WarnContext(r.Context(), "request resulted with client error", attrs...)
		} else {
			slog.InfoContext(r.Context(), "request finished successfully", attrs...)
		}
	})
}

func CorrelationHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := GetCorrelationIDReq(r)
		r = r.WithContext(SetCorrelationID(r.Context(), correlationID))
		next.ServeHTTP(w, r)
	})
}

type metricsHttpWriter struct {
	http.ResponseWriter
	http.Flusher
	http.Hijacker
	statusCode int
}

func (m *metricsHttpWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	m.ResponseWriter.WriteHeader(statusCode)
}
