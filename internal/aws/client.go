package aws

import (
	"context"
	"fmt"
	"sync"

	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type MultiRegionClient struct {
	configs        map[string]awsSDK.Config
	ec2Clients     sync.Map
	ssmClients     sync.Map
	secretsClients sync.Map
}

func NewMultiRegionClient(ctx context.Context, profile string, regions []string) (*MultiRegionClient, error) {
	cfgMap := make(map[string]awsSDK.Config, len(regions))
	for _, region := range regions {
		var opts []func(*config.LoadOptions) error
		opts = append(opts, config.WithRegion(region))
		if profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(profile))
		}
		cfg, err := config.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("load aws config for region %s: %w", region, err)
		}
		cfgMap[region] = cfg
	}
	return &MultiRegionClient{configs: cfgMap}, nil
}

func (c *MultiRegionClient) EC2(region string) (*ec2.Client, error) {
	if v, ok := c.ec2Clients.Load(region); ok {
		return v.(*ec2.Client), nil
	}
	cfg, ok := c.configs[region]
	if !ok {
		return nil, fmt.Errorf("region %s is not configured", region)
	}
	client := ec2.NewFromConfig(cfg)
	c.ec2Clients.Store(region, client)
	return client, nil
}

func (c *MultiRegionClient) SSM(region string) (*ssm.Client, error) {
	if v, ok := c.ssmClients.Load(region); ok {
		return v.(*ssm.Client), nil
	}
	cfg, ok := c.configs[region]
	if !ok {
		return nil, fmt.Errorf("region %s is not configured", region)
	}
	client := ssm.NewFromConfig(cfg)
	c.ssmClients.Store(region, client)
	return client, nil
}

func (c *MultiRegionClient) SecretsManager(region string) (*secretsmanager.Client, error) {
	if v, ok := c.secretsClients.Load(region); ok {
		return v.(*secretsmanager.Client), nil
	}
	cfg, ok := c.configs[region]
	if !ok {
		return nil, fmt.Errorf("region %s is not configured", region)
	}
	client := secretsmanager.NewFromConfig(cfg)
	c.secretsClients.Store(region, client)
	return client, nil
}
