package storage

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/bradhe/what-day-is-it/pkg/models"
)

type dynamodbPhoneNumberManager struct {
	tablePrefix string
	svc         *dynamodb.DynamoDB
}

func (m dynamodbPhoneNumberManager) tableName() string {
	return m.tablePrefix + "-PhoneNumbers"
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "0"
	}

	return fmt.Sprintf("%d", t.UTC().Unix())
}

func getString(name string, attrs map[string]*dynamodb.AttributeValue) string {
	if val, ok := attrs[name]; ok {
		return aws.StringValue(val.S)
	}

	return ""
}

func getStringAttribute(val string) *dynamodb.AttributeValue {
	var attr dynamodb.AttributeValue
	attr.S = aws.String(val)
	return &attr
}

func getTime(name string, attrs map[string]*dynamodb.AttributeValue) *time.Time {
	if val, ok := attrs[name]; ok {
		str := aws.StringValue(val.N)
		i, _ := strconv.ParseInt(str, 10, 64)

		t := time.Unix(i, 0)
		return &t
	}

	return &time.Time{}
}

func getTimeAttribute(t *time.Time) *dynamodb.AttributeValue {
	var attr dynamodb.AttributeValue
	attr.N = aws.String(formatTime(t))
	return &attr
}

func getBool(name string, attrs map[string]*dynamodb.AttributeValue) bool {
	if val, ok := attrs[name]; ok {
		return aws.BoolValue(val.BOOL)
	}

	return false
}

func getBoolAttribute(b bool) *dynamodb.AttributeValue {
	var attr dynamodb.AttributeValue
	attr.BOOL = aws.Bool(b)
	return &attr
}

func deserializePhoneNumber(attrs map[string]*dynamodb.AttributeValue) (num models.PhoneNumber) {
	num.Number = getString("phone_number", attrs)
	num.Timezone = getString("timezone", attrs)
	num.LastSentAt = getTime("last_sent_at", attrs)
	num.IsSendable = getBool("is_sendable", attrs)
	num.SendDeadline = getTime("send_deadline", attrs)
	return
}

func deserializeAllPhoneNumbers(arr []map[string]*dynamodb.AttributeValue) (out []models.PhoneNumber) {
	for _, attrs := range arr {
		out = append(out, deserializePhoneNumber(attrs))
	}

	return out
}

func (m dynamodbPhoneNumberManager) GetNBySendDeadline(n int, deadline *time.Time) ([]models.PhoneNumber, error) {
	in := dynamodb.ScanInput{
		TableName: aws.String(m.tableName()),
		ExpressionAttributeNames: map[string]*string{
			"#deadline": aws.String("send_deadline"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":deadline": &dynamodb.AttributeValue{
				N: aws.String(formatTime(deadline)),
			},
		},
		FilterExpression: aws.String("#deadline < :deadline"),
		Limit:            aws.Int64(int64(n)),
	}

	if out, err := m.svc.Scan(&in); err != nil {
		log.Printf("ERR failed to scan for %d phone numbers in DynamoDB: %s", n, err.Error())
		return nil, err
	} else {
		return deserializeAllPhoneNumbers(out.Items), nil
	}
}

func (m dynamodbPhoneNumberManager) Get(num string) (models.PhoneNumber, error) {
	in := dynamodb.GetItemInput{
		TableName: aws.String(m.tableName()),
		Key: map[string]*dynamodb.AttributeValue{
			"phone_number": getStringAttribute(num),
		},
		ConsistentRead: aws.Bool(true),
	}

	if out, err := m.svc.GetItem(&in); err != nil {
		log.Printf("ERR failed to get phone number in DynamoDB: %s", err.Error())
		return models.PhoneNumber{}, err
	} else {
		return deserializePhoneNumber(out.Item), nil
	}
}

func nextDeadline(sentAt *time.Time, loc *time.Location) *time.Time {
	nextDay := sentAt.In(loc).Add(24 * time.Hour)

	// this is a funny way of thunking the date to 8am in that location.
	deadline := time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 8, 0, 0, 0, nextDay.Location())

	return &deadline
}

func (m dynamodbPhoneNumberManager) UpdateSent(num *models.PhoneNumber, sentAt *time.Time) error {
	newDeadline := nextDeadline(sentAt, MustLoadLocation(num.Timezone))

	in := dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"phone_number": getStringAttribute(num.Number),
		},
		TableName: aws.String(m.tableName()),
		ExpressionAttributeNames: map[string]*string{
			"#send_deadline": aws.String("send_deadline"),
			"#last_sent_at":  aws.String("last_sent_at"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":send_deadline": getTimeAttribute(newDeadline),
			":last_sent_at":  getTimeAttribute(sentAt),
		},
		UpdateExpression: aws.String("SET #send_deadline = :send_deadline, #last_sent_at = :last_sent_at"),
	}

	if _, err := m.svc.UpdateItem(&in); err != nil {
		log.Printf("ERR failed to updating sent phone number in DynamoDB: %s", err.Error())
		return err
	} else {
		num.LastSentAt = sentAt
		num.SendDeadline = newDeadline
	}

	return nil
}

func (m dynamodbPhoneNumberManager) UpdateNotSendable(num *models.PhoneNumber) error {
	in := dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"phone_number": getStringAttribute(num.Number),
		},
		TableName: aws.String(m.tableName()),
		ExpressionAttributeNames: map[string]*string{
			"#is_sendable": aws.String("is_sendable"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":is_sendable": getBoolAttribute(false),
		},
		UpdateExpression: aws.String("SET #is_sendable = :is_sendable"),
	}

	if _, err := m.svc.UpdateItem(&in); err != nil {
		log.Printf("ERR failed to updating not sendable number in DynamoDB: %s", err.Error())
		return err
	} else {
		num.IsSendable = false
	}

	return nil
}

func (m dynamodbPhoneNumberManager) UpdateSendable(num *models.PhoneNumber) error {
	in := dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"phone_number": getStringAttribute(num.Number),
		},
		TableName: aws.String(m.tableName()),
		ExpressionAttributeNames: map[string]*string{
			"#is_sendable": aws.String("is_sendable"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":is_sendable": getBoolAttribute(true),
		},
		UpdateExpression: aws.String("SET #is_sendable = :is_sendable"),
	}

	if _, err := m.svc.UpdateItem(&in); err != nil {
		log.Printf("ERR failed to updating sendable number in DynamoDB: %s", err.Error())
		return err
	} else {
		num.IsSendable = false
	}

	return nil
}

func (m dynamodbPhoneNumberManager) UpdateSkipped(num *models.PhoneNumber, sentAt *time.Time) error {
	newDeadline := nextDeadline(sentAt, MustLoadLocation(num.Timezone))

	in := dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"phone_number": getStringAttribute(num.Number),
		},
		TableName: aws.String(m.tableName()),
		ExpressionAttributeNames: map[string]*string{
			"#send_deadline": aws.String("send_deadline"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":send_deadline": getTimeAttribute(newDeadline),
		},
		UpdateExpression: aws.String("SET #send_deadline = :send_deadline"),
	}

	if _, err := m.svc.UpdateItem(&in); err != nil {
		log.Printf("ERR failed to updating skipped phone number in DynamoDB: %s", err.Error())
		return err
	} else {
		num.SendDeadline = newDeadline
	}

	return nil
}

func MustLoadLocation(str string) *time.Location {
	loc, _ := time.LoadLocation(str)
	return loc
}

func serializePhoneNumber(num models.PhoneNumber) map[string]*dynamodb.AttributeValue {
	return map[string]*dynamodb.AttributeValue{
		"phone_number":  getStringAttribute(num.Number),
		"timezone":      getStringAttribute(num.Timezone),
		"last_sent_at":  getTimeAttribute(num.LastSentAt),
		"is_sendable":   getBoolAttribute(num.IsSendable),
		"send_deadline": getTimeAttribute(num.SendDeadline),
	}
}

func (m dynamodbPhoneNumberManager) Create(num models.PhoneNumber) error {
	in := dynamodb.PutItemInput{
		TableName:           aws.String(m.tableName()),
		Item:                serializePhoneNumber(num),
		ConditionExpression: aws.String("attribute_not_exists(phone_number)"),
	}

	if _, err := m.svc.PutItem(&in); err != nil {
		// Let's see if we can figure out what type of error this is.
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "ConditionalCheckFailedException":
				log.Printf("ERR phone number already subscribed")
				return ErrRecordExists
			}

			return err
		} else {
			log.Printf("ERR failed to put phone numbers in DynamoDB: %s", err.Error())
			return err
		}
	}

	return nil
}

type dynamodbManagers struct {
	tablePrefix string
	svc         *dynamodb.DynamoDB
}

func (m dynamodbManagers) PhoneNumbers() PhoneNumberManager {
	return &dynamodbPhoneNumberManager{
		tablePrefix: m.tablePrefix,
		svc:         m.svc,
	}
}

func newDynamoDBManagers(tablePrefix string) Managers {
	sess := newAWSSession()
	svc := dynamodb.New(sess)

	return &dynamodbManagers{
		tablePrefix: tablePrefix,
		svc:         svc,
	}
}
