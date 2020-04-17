package storage

import (
	"time"

	"github.com/bradhe/what-day-is-it/pkg/models"
)

type PhoneNumberManager interface {
	GetNBySendDeadline(int, *time.Time) ([]models.PhoneNumber, error)
	UpdateSent(*models.PhoneNumber, *time.Time) error
	UpdateSkipped(*models.PhoneNumber, *time.Time) error
	UpdateNotSendable(*models.PhoneNumber) error
	UpdateSendable(*models.PhoneNumber) error
	Create(models.PhoneNumber) error
	Get(string) (models.PhoneNumber, error)
}

type Managers interface {
	PhoneNumbers() PhoneNumberManager
}

func New(tablePrefix string) Managers {
	return newDynamoDBManagers(tablePrefix)
}
