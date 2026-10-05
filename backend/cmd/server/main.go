package main

import (
	"log"
	"net/http"
	"sn-backend/internal/db/sqlite"
	"sn-backend/internal/handler"
	"sn-backend/internal/middleware"
	"sn-backend/internal/repository"
	"sn-backend/internal/server"
	"time"
)

func main() {
	if err := sqlite.InitDB("sn.db"); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	repositories := repository.New(sqlite.DB)
	server.RegisterRoutes(mux, handler.New(repositories))

	// timeouts so slow or stuck clients cannot hold connections open forever
	// (no WriteTimeout: websockets stay open, they keep their own deadlines)
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           middleware.SecurityHeaders(middleware.RateLimit(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // room for 3 images of 10 MB
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
