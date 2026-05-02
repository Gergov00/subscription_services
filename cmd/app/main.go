package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	v1 "subscriptionServices/internal/delivery/http/v1"
	"subscriptionServices/internal/repository/postgres"
	"subscriptionServices/internal/service"

	_ "subscriptionServices/docs"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/fx"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Сервис Подписок API
// @version 1.0
// @description REST API для агрегации данных об онлайн подписках пользователей.
// @host localhost:8080
// @BasePath /api/v1
func main() {
	app := fx.New(
		fx.Provide(
			NewLogger,
			NewRouter,
			NewDatabase,
			postgres.NewSubscriptionStorage,
			service.NewSubscriptionService,
			v1.NewSubscriptionHandler,
		),
		fx.Invoke(registerHooks),
	)

	app.Run()
}

func registerHooks(
	handler *v1.SubscriptionHandler,
	router *gin.Engine,
	log *slog.Logger,
	lc fx.Lifecycle,
) {
	router.Use(v1.RequestLogger(log))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	{
		subs := api.Group("/subscriptions")
		{
			subs.POST("", handler.Create)
			subs.GET("", handler.List)
			subs.GET("/cost", handler.CalculateCost)
			subs.GET("/:id", handler.GetByID)
			subs.PATCH("/:id", handler.Update)
			subs.DELETE("/:id", handler.Delete)
		}
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			fmt.Println("Сервер запущен через uber/fx на порту 8080...")
			fmt.Println("Swagger UI доступен по адресу: http://localhost:8080/swagger/index.html")
			go server.ListenAndServe()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("Сервер останавливается...")
			return server.Shutdown(ctx)
		},
	})
}

func NewDatabase(lc fx.Lifecycle) (*pgxpool.Pool, error) {
	_ = godotenv.Load()
	
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	
	if user == "" { user = "user" }
	if password == "" { password = "password" }
	if host == "" { host = "localhost" }
	if port == "" { port = "5432" }
	if dbName == "" { dbName = "subscriptions" }

	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbName)
	
	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			fmt.Println("База данных подключается...")
			return pool.Ping(ctx)
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("База данных закрывается...")
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func NewRouter(log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	return router
}
