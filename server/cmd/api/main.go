package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/app"
	"github.com/shuichirohayafuji/spendable-today/server/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "withdrawal-batch" && os.Args[1] != "withdrawal-batch-health") {
			log.Fatal("usage: spendable-today [migrate|withdrawal-batch|withdrawal-batch-health]")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if os.Args[1] == "withdrawal-batch-health" {
			if err := app.RefreshWithdrawalBatchHealth(ctx, cfg); err != nil {
				log.Fatal("withdrawal health check failed")
			}
			return
		}
		if os.Args[1] == "withdrawal-batch" {
			if err := app.RunWithdrawalBatch(ctx, cfg, time.Now()); err != nil {
				log.Fatal("withdrawal batch incomplete")
			}
			log.Print("withdrawal batch completed")
			return
		}
		if err := app.Migrate(ctx, cfg); err != nil {
			log.Fatal("database migration failed: ", err)
		}
		log.Print("database migrations applied")
		return
	}
	if cfg.UseMockLLM {
		log.Printf("LLM mode: mock")
	} else if cfg.OpenAIAPIKey == "" {
		log.Printf("LLM mode: OpenAI requested, but OPENAI_API_KEY is empty; responses will use fallback")
	} else {
		log.Printf("LLM mode: OpenAI Responses API (model=%s, reasoning_effort=%s)", cfg.OpenAIModel, cfg.OpenAIReasoningEffort)
	}
	application, err := app.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("initialize application: %v", err)
	}
	defer application.Close()
	server := application.Server

	go func() {
		log.Printf("Spendable Today API listening on http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
