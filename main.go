package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/iantal/rm/internal/repository"
	"github.com/iantal/rm/internal/util"

	gohandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/iantal/rm/internal/files"
	"github.com/iantal/rm/internal/rest/handlers"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" // postgres
	"github.com/spf13/viper"
)

func main() {
	viper.AutomaticEnv()
	logger := util.NewLogger()

	bp := fmt.Sprintf("%v", viper.Get("BASE_PATH"))
	rkHost := fmt.Sprintf("%v", viper.Get("RK_HOST"))

	// create the storage class, use local storage
	// max filesize 5GB
	stor, err := files.NewLocal(logger, bp, 1024*1000*1000*5)
	if err != nil {
		logger.WithField("error", err).Error("Unable to create storage")
		os.Exit(1)
	}

	user := viper.Get("POSTGRES_USER")
	password := viper.Get("POSTGRES_PASSWORD")
	database := viper.Get("POSTGRES_DB")
	host := viper.Get("POSTGRES_HOST")
	port := viper.Get("POSTGRES_PORT")
	connection := fmt.Sprintf("host=%v port=%v user=%v dbname=%v password=%v sslmode=disable", host, port, user, database, password)

	db, err := gorm.Open("postgres", connection)
	defer db.Close()
	if err != nil {
		panic("Failed to connect to database!")
	}

	err = db.DB().Ping()
	if err != nil {
		panic("Ping failed!")
	}

	projectDB := repository.NewProjectDB(logger, db)
	projH := handlers.NewProjects(logger, stor, projectDB, rkHost)
	// mw := handlers.GzipHandler{}

	// create a new serve mux and register the handlers
	sm := mux.NewRouter()

	ch := gohandlers.CORS(gohandlers.AllowedOrigins([]string{"*"}))

	gh := sm.Methods(http.MethodGet).Subrouter()
	gh.HandleFunc("/api/v1/projects/{id:[0-9a-f-]{36}}/{commit:[0-9a-f]{40}}/download", projH.Download)

	// create a new server
	s := http.Server{
		Addr:         ":8005",            // configure the bind address
		Handler:      ch(sm),             // set the default handler
		ReadTimeout:  5 * time.Second,    // max time to read request from the client
		WriteTimeout: 1000 * time.Second, // max time to write response to the client
		IdleTimeout:  1200 * time.Second, // max time for connections using TCP Keep-Alive
	}

	// start the server
	go func() {
		logger.Info("Starting server bind_address :8005")
		err := s.ListenAndServe()
		if err != nil {
			logger.WithField("error", err).Error("Unable to start server")
			os.Exit(1)
		}
	}()

	// trap sigterm or interupt and gracefully shutdown the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	// Block until a signal is received.
	sig := <-c
	logger.WithField("signal", sig).Info("Shutting down server with signal")

	// gracefully shutdown the server, waiting max 30 seconds for current operations to complete
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	s.Shutdown(ctx)
}
