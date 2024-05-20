package internal

import (
	"context"
	"log"
	"net/url"
	"path/filepath"
	"time"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPgAndMigrations() (string, func(), error) {
	testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)
	ctx := context.Background()
	nw, err := network.New(ctx)
	if err != nil {
		return "", nil, err
	}
	pgContainer, err := postgres.RunContainer(
		ctx,
		testcontainers.WithImage("postgres:15-alpine3.17"),
		postgres.WithInitScripts("../.docker/db/001_create_user_db.sql"),
		network.WithNetwork([]string{"pg"}, nw),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		return "", nil, err
	}
	connString, err := pgContainer.ConnectionString(ctx, "sslmode=disable", "search_path=ventrata")
	if err != nil {
		return "", nil, err
	}

	urlForTest, err := url.Parse(connString)
	if err != nil {
		return "", nil, err
	}
	urlForMigration, err := url.Parse(connString)
	if err != nil {
		return "", nil, err
	}
	urlForTest.User = url.UserPassword("ventrata_usr", "ventrata123")
	urlForMigration.User = url.UserPassword("ventrata_usr", "ventrata123")
	urlForTest.Path = "ventrata"
	urlForMigration.Path = "ventrata"
	urlForMigration.Host = "pg:5432"

	migrationDirectory, err := filepath.Abs(filepath.Join("..", "migrations"))
	if err != nil {
		return "", nil, err
	}
	migrations, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "migrate/migrate",
			WaitingFor: wait.ForExit(),
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      migrationDirectory,
					ContainerFilePath: "/migrations",
				},
			},
			Cmd:      []string{"-path", "/migrations/", "-database", urlForMigration.String(), "up"},
			Networks: []string{nw.Name},
		},
		Started: true,
	})
	if err != nil {
		return "", nil, err
	}
	return urlForTest.String(), func() {
		_ = migrations.Terminate(ctx)
		_ = pgContainer.Terminate(ctx)
		_ = nw.Remove(ctx)
	}, nil
}
