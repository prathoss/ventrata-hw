package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func ServeWithShutdown(s *http.Server) error {
	ctx := context.Background()
	ctx, cFunc := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cFunc()

	errChan := make(chan error, 1)

	go func() {
		slog.Info("Server is running", "address", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("Server is shutting down")
		shutdownCtx, shutdownCFunc := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCFunc()
		if err := s.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
	case err := <-errChan:
		return fmt.Errorf("server listen and serve failed: %w", err)
	}
	return nil
}

type HttpHandler func(w http.ResponseWriter, r *http.Request) (any, error)

func (f HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	responseModel, err := f(w, r)
	if err != nil {
		if problemWriter, ok := err.(HttpProblemWriter); ok {
			if err := problemWriter.WriteProblem(r.Context(), w); err != nil {
				slog.ErrorContext(r.Context(), "response could not be written", Err(err))
			}
		} else {
			internalServerError := NewInternalServerError(err)
			err := internalServerError.WriteProblem(r.Context(), w)
			if err != nil {
				slog.ErrorContext(r.Context(), "response could not be written", Err(err))
			}
		}
		return
	}

	if responseModel == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responseModel); err != nil {
		slog.ErrorContext(r.Context(), "could not encode response body", Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type ProblemDetail struct {
	Status int    `json:"status"`
	Type   string `json:"type"`
	Title  string `json:"title"`
}

type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ValidationProblemDetail struct {
	ProblemDetail
	InvalidParams []InvalidParam `json:"invalid-params"`
}

type HttpProblemWriter interface {
	WriteProblem(ctx context.Context, w http.ResponseWriter) error
}

var _ error = &ServiceUnavailableError{}
var _ HttpProblemWriter = &ServiceUnavailableError{}

func NewServiceUnavailableError(err error) *ServiceUnavailableError {
	return &ServiceUnavailableError{
		innerError: err,
	}
}

type ServiceUnavailableError struct {
	innerError error
}

func (s *ServiceUnavailableError) WriteProblem(ctx context.Context, w http.ResponseWriter) error {
	slog.ErrorContext(ctx, "request resulted in a service unavailable", Err(s.innerError))
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Header().Set("Content-Type", "application/problem+json")
	detail := ProblemDetail{
		Status: http.StatusServiceUnavailable,
		Type:   "https://datatracker.ietf.org/doc/html/rfc7231#section-6.6.4",
		Title:  "The server is unavailable",
	}
	return json.NewEncoder(w).Encode(detail)
}

func (s *ServiceUnavailableError) Error() string {
	return s.innerError.Error()
}

var _ error = &BadRequestError{}
var _ HttpProblemWriter = &BadRequestError{}

func NewBadRequestError(invalidParams ...InvalidParam) *BadRequestError {
	return &BadRequestError{invalidParams: invalidParams}
}

type BadRequestError struct {
	invalidParams []InvalidParam
}

func (b *BadRequestError) Error() string {
	return "Request is invalid"
}

func (b *BadRequestError) WriteProblem(_ context.Context, w http.ResponseWriter) error {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/problem+json")
	detail := ValidationProblemDetail{
		ProblemDetail: ProblemDetail{
			Status: http.StatusBadRequest,
			Type:   "https://datatracker.ietf.org/doc/html/rfc7231#section-6.5.1",
			Title:  "Request parameters did not validate",
		},
		InvalidParams: b.invalidParams,
	}
	return json.NewEncoder(w).Encode(detail)
}

var _ error = &InternalServerError{}
var _ HttpProblemWriter = &InternalServerError{}

func NewInternalServerError(err error) *InternalServerError {
	return &InternalServerError{
		innerError: err,
	}
}

type InternalServerError struct {
	innerError error
}

func (i *InternalServerError) Error() string {
	return i.innerError.Error()
}

func (i *InternalServerError) WriteProblem(ctx context.Context, w http.ResponseWriter) error {
	slog.ErrorContext(ctx, "request resulted in a internal server error", Err(i.innerError))
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/problem+json")
	detail := ProblemDetail{
		Status: http.StatusInternalServerError,
		Type:   "https://datatracker.ietf.org/doc/html/rfc7231#section-6.6.1",
		Title:  "Internal Server Error",
	}
	return json.NewEncoder(w).Encode(detail)
}
