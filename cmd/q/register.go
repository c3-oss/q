package main

// Blank imports register each database backend's factory via its init().
import (
	_ "github.com/c3-oss/q/internal/adapter/dynamodb"
	_ "github.com/c3-oss/q/internal/adapter/mongo"
	_ "github.com/c3-oss/q/internal/adapter/mysql"
	_ "github.com/c3-oss/q/internal/adapter/postgres"
	_ "github.com/c3-oss/q/internal/adapter/redis"
	_ "github.com/c3-oss/q/internal/adapter/sqlite"
)
