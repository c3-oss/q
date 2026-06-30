//go:build integration

package itest

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcdynamodb "github.com/testcontainers/testcontainers-go/modules/dynamodb"
)

func TestDynamoDBIntegration(t *testing.T) {
	ctx := context.Background()
	ctr, err := tcdynamodb.Run(ctx, "amazon/dynamodb-local:2.5.2")
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	hostPort, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
	)
	require.NoError(t, err)
	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String("http://" + hostPort)
	})

	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:            aws.String("Users"),
		BillingMode:          types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS}},
		KeySchema:            []types.KeySchemaElement{{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash}},
	})
	require.NoError(t, err)
	require.NoError(t, dynamodb.NewTableExistsWaiter(client).Wait(ctx,
		&dynamodb.DescribeTableInput{TableName: aws.String("Users")}, 30*time.Second))

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("Users"),
		Item: map[string]types.AttributeValue{
			"id":    &types.AttributeValueMemberS{Value: "u-42"},
			"email": &types.AttributeValueMemberS{Value: "ada@example.com"},
		},
	})
	require.NoError(t, err)

	ad := open(t, ctx, "dynamodb://"+hostPort+"?region=us-east-1")

	recs := readAll(t, ctx, ad, `SELECT id, email FROM "Users" WHERE id = 'u-42'`)
	require.Len(t, recs, 1)
	email, _ := fieldNamed(recs[0], "email")
	require.Equal(t, "ada@example.com", email)

	assertRejected(t, ctx, ad, `DELETE FROM "Users" WHERE id = 'u-42'`)
}
