package sms

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSConfig configures AWS SNS as the SMS transport. Credentials are resolved by the AWS
// SDK's default chain (env AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, or an IAM role).
type SNSConfig struct {
	Region   string
	SenderID string // optional; only where the destination country permits alphanumeric senders
	SMSType  string // "Transactional" (default — OTP) or "Promotional"
}

var (
	snsOnce   sync.Once
	snsClient *sns.Client
	snsInErr  error
)

// SendSNS publishes a single SMS via SNS (Zitadel already generated/verified the OTP; SNS
// is only the transport). The client is built once and reused.
func SendSNS(ctx context.Context, c SNSConfig, phone, text string) error {
	snsOnce.Do(func() {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(c.Region))
		if err != nil {
			snsInErr = fmt.Errorf("aws config: %w", err)
			return
		}
		snsClient = sns.NewFromConfig(cfg)
	})
	if snsInErr != nil {
		return snsInErr
	}

	smsType := c.SMSType
	if smsType == "" {
		smsType = "Transactional"
	}
	attrs := map[string]snstypes.MessageAttributeValue{
		"AWS.SNS.SMS.SMSType": {DataType: aws.String("String"), StringValue: aws.String(smsType)},
	}
	if c.SenderID != "" {
		attrs["AWS.SNS.SMS.SenderID"] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(c.SenderID)}
	}

	_, err := snsClient.Publish(ctx, &sns.PublishInput{
		PhoneNumber:       aws.String(phone),
		Message:           aws.String(text),
		MessageAttributes: attrs,
	})
	if err != nil {
		return fmt.Errorf("sns publish: %w", err)
	}
	return nil
}
