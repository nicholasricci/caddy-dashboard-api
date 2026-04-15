package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

type EC2Service struct {
	clients *MultiRegionClient
}

func NewEC2Service(clients *MultiRegionClient) *EC2Service {
	return &EC2Service{clients: clients}
}

func (s *EC2Service) DiscoverByTag(ctx context.Context, region, tagKey, tagValue string) ([]models.CaddyNode, error) {
	client, err := s.clients.EC2(region)
	if err != nil {
		return nil, err
	}

	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: aws.String(fmt.Sprintf("tag:%s", tagKey)), Values: []string{tagValue}},
			{Name: aws.String("instance-state-name"), Values: []string{"running", "stopped"}},
		},
	}

	out, err := client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describe instances: %w", err)
	}

	now := time.Now().UTC()
	nodes := make([]models.CaddyNode, 0)
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			name := extractTag(inst.Tags, "Name")
			instanceID := aws.ToString(inst.InstanceId)
			privateIP := aws.ToString(inst.PrivateIpAddress)
			node := models.CaddyNode{
				Name:       fallback(name, instanceID),
				InstanceID: optionalString(instanceID),
				PrivateIP:  optionalString(privateIP),
				Region:     region,
				SSMEnabled: true,
				Status:     string(inst.State.Name),
				LastSeenAt: &now,
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func extractTag(tags []types.Tag, key string) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == key {
			return aws.ToString(t.Value)
		}
	}
	return ""
}

func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func fallback(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return "caddy-node"
}
