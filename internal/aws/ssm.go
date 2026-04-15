package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

type SSMService struct {
	clients *MultiRegionClient
}

type CommandResult struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
}

func NewSSMService(clients *MultiRegionClient) *SSMService {
	return &SSMService{clients: clients}
}

func (s *SSMService) RunShellCommand(ctx context.Context, region, instanceID, command string) (*CommandResult, error) {
	client, err := s.clients.SSM(region)
	if err != nil {
		return nil, err
	}

	sendOut, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{instanceID},
		Parameters: map[string][]string{
			"commands": {command},
		},
		Comment:        aws.String("caddy-dashboard command"),
		TimeoutSeconds: aws.Int32(120),
	})
	if err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	cmdID := aws.ToString(sendOut.Command.CommandId)
	deadline := time.Now().Add(2 * time.Minute)
	interval := 2 * time.Second

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("ssm command timed out")
		}
		inv, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
			CommandId:  aws.String(cmdID),
			InstanceId: aws.String(instanceID),
		})
		if err == nil {
			status := string(inv.Status)
			if status == string(types.CommandInvocationStatusSuccess) ||
				status == string(types.CommandInvocationStatusFailed) ||
				status == string(types.CommandInvocationStatusCancelled) ||
				status == string(types.CommandInvocationStatusTimedOut) {
				return &CommandResult{
					CommandID: cmdID,
					Status:    status,
					Stdout:    aws.ToString(inv.StandardOutputContent),
					Stderr:    aws.ToString(inv.StandardErrorContent),
				}, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// DiscoverManagedInstances lists instances registered with SSM in the given region (online + offline).
func (s *SSMService) DiscoverManagedInstances(ctx context.Context, region string) ([]models.CaddyNode, error) {
	client, err := s.clients.SSM(region)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var nodes []models.CaddyNode
	var token *string
	for {
		out, err := client.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
			MaxResults: aws.Int32(50),
			NextToken:  token,
		})
		if err != nil {
			return nil, fmt.Errorf("describe instance information: %w", err)
		}
		for _, info := range out.InstanceInformationList {
			id := aws.ToString(info.InstanceId)
			if id == "" {
				continue
			}
			ip := aws.ToString(info.IPAddress)
			var ipPtr *string
			if ip != "" {
				ipPtr = aws.String(ip)
			}
			name := id
			if info.ComputerName != nil && aws.ToString(info.ComputerName) != "" {
				name = aws.ToString(info.ComputerName)
			}
			nodes = append(nodes, models.CaddyNode{
				Name:         name,
				InstanceID:   aws.String(id),
				PrivateIP:    ipPtr,
				Region:       region,
				SSMEnabled:   true,
				Status:       string(info.PingStatus),
				LastSeenAt:   &now,
			})
		}
		token = out.NextToken
		if token == nil || aws.ToString(token) == "" {
			break
		}
	}
	return nodes, nil
}
