package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretsService struct {
	clients *MultiRegionClient
}

func NewSecretsService(clients *MultiRegionClient) *SecretsService {
	return &SecretsService{clients: clients}
}

func (s *SecretsService) GetSecretString(ctx context.Context, region, secretARN string) (string, error) {
	client, err := s.clients.SecretsManager(region)
	if err != nil {
		return "", err
	}
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretARN),
	})
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}
	return aws.ToString(out.SecretString), nil
}

func (s *SecretsService) GetUsersMap(ctx context.Context, region, secretARN string) (map[string]string, error) {
	raw, err := s.GetSecretString(ctx, region, secretARN)
	if err != nil {
		return nil, err
	}
	users := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &users); err != nil {
		return nil, fmt.Errorf("users secret must be a JSON object map[user]=bcrypt_hash: %w", err)
	}
	return users, nil
}
