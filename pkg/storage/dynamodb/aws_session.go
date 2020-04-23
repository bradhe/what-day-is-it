package dynamodb

import (
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
)

func newAWSSession() *session.Session {
	return session.Must(session.NewSession(defaults.Get().Config))
}
