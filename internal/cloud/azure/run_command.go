package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// ShellCommandResult mirrors AWS SSM / GCP OS Config for the Caddy executors.
type ShellCommandResult struct {
	Status string
	Stdout string
	Stderr string
	Meta   map[string]string
}

// RunCommandRunner executes managed VM Run Command (Linux: RunShellScript).
type RunCommandRunner struct {
	cred azcore.TokenCredential
	poll time.Duration
}

// NewRunCommandRunner returns nil, nil when disabled.
func NewRunCommandRunner(ctx context.Context, enabled bool, pollInterval time.Duration) (*RunCommandRunner, error) {
	if !enabled {
		return nil, nil
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	_ = ctx
	return &RunCommandRunner{cred: cred, poll: pollInterval}, nil
}

// RunShellTarget identifies the VM.
type RunShellTarget struct {
	SubscriptionID string
	ResourceGroup  string
	VMName         string
	Timeout        time.Duration
}

// RunShellCommand runs a shell script on the VM via RunShellScript.
func (r *RunCommandRunner) RunShellCommand(ctx context.Context, t RunShellTarget, script string) (*ShellCommandResult, error) {
	if r == nil || r.cred == nil {
		return nil, fmt.Errorf("azure run command runner not initialized")
	}
	sub := strings.TrimSpace(t.SubscriptionID)
	rg := strings.TrimSpace(t.ResourceGroup)
	vm := strings.TrimSpace(t.VMName)
	if sub == "" || rg == "" || vm == "" {
		return nil, fmt.Errorf("azure run command: subscription_id, resource_group, and vm_name are required")
	}
	client, err := armcompute.NewVirtualMachinesClient(sub, r.cred, nil)
	if err != nil {
		return nil, fmt.Errorf("virtual machines client: %w", err)
	}
	deadline := time.Now().Add(r.timeout(t.Timeout))
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	cmdID := "RunShellScript"
	in := armcompute.RunCommandInput{
		CommandID: to.Ptr(cmdID),
		Script:    splitScriptLines(script),
	}
	poller, err := client.BeginRunCommand(ctx, rg, vm, in, nil)
	if err != nil {
		return nil, fmt.Errorf("begin run command: %w", err)
	}
	resp, err := poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: r.poll})
	if err != nil {
		return nil, fmt.Errorf("run command poll: %w", err)
	}
	var stdout, stderr strings.Builder
	if resp.Value != nil {
		for _, st := range resp.Value {
			if st == nil {
				continue
			}
			if st.Message != nil {
				stdout.WriteString(strings.TrimSpace(*st.Message))
				stdout.WriteByte('\n')
			}
			if st.DisplayStatus != nil {
				stderr.WriteString(strings.TrimSpace(*st.DisplayStatus))
				stderr.WriteByte('\n')
			}
		}
	}
	return &ShellCommandResult{
		Status: "Success",
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
		Meta: map[string]string{
			"azure_subscription_id": sub,
			"azure_resource_group":  rg,
			"azure_vm_name":         vm,
			"azure_command_id":      cmdID,
		},
	}, nil
}

func (r *RunCommandRunner) timeout(t time.Duration) time.Duration {
	if t > 0 {
		return t
	}
	return 3 * time.Minute
}

func splitScriptLines(script string) []*string {
	s := strings.ReplaceAll(script, "\r\n", "\n")
	parts := strings.Split(s, "\n")
	out := make([]*string, 0, len(parts))
	for _, p := range parts {
		out = append(out, to.Ptr(p))
	}
	if len(out) == 0 {
		return []*string{to.Ptr("true")}
	}
	return out
}
