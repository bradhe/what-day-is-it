package storage

import (
	"github.com/bradhe/what-day-is-it/pkg/storage/dynamodb"
	"github.com/bradhe/what-day-is-it/pkg/storage/managers"
)

func New(tablePrefix string) managers.Managers {
	return dynamodb.New(tablePrefix)
}
