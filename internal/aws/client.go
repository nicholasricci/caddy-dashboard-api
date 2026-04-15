package aws

import (
	"context"
	"fmt"

	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type MultiRegionClient struct {
	configs map[string]awsSDK.Config
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
	cfg, ok := c.configs[region]
	if !ok {
		return nil, fmt.Errorf("region %s is not configured", region)
	}
	return ec2.NewFromConfig(cfg), nil
}

func (c *MultiRegionClient) SSM(region string) (*ssm.Client, error) {
	cfg, ok := c.configs[region]
	if !ok {
		return nil, fmt.Errorf("region %s is not configured", region)
	}
	return ssm.NewFromConfig(cfg), nil
}

func (c *MultiRegionClient) SecretsManager(region string) (*secretsmanager.Client, error) {
	cfg, ok := c.configs[region]
	if !ok {
		return nil, fmt.Errorf("region %s is not configured", region)
	}
	return secretsmanager.NewFromConfig(cfg), nil
}
