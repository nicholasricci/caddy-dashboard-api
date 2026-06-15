package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	osconfig "cloud.google.com/go/osconfig/apiv1"
	"cloud.google.com/go/osconfig/apiv1/osconfigpb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	osPolicyIDForCaddy      = "caddy-dash-policy"
	osPolicyResourceID      = "caddy-dash-exec"
	defaultAssignmentPrefix = "caddy-dash"
)

// ShellCommandResult is the outcome of a guest shell run via OS Config (same shape as AWS SSM for callers).
type ShellCommandResult struct {
	Status string
	Stdout string
	Stderr string
	Meta   map[string]string
}

// OSConfigShellRunner runs a shell script on targeted VMs using a temporary OS policy assignment.
type OSConfigShellRunner struct {
	client           *osconfig.OsConfigZonalClient
	defaultTimeout   time.Duration
	assignmentPrefix string
}

// NewOSConfigShellRunner returns nil, nil when the OS Config client cannot be constructed (no ADC, etc.).
func NewOSConfigShellRunner(ctx context.Context, defaultTimeout time.Duration, assignmentPrefix string) (*OSConfigShellRunner, error) {
	if defaultTimeout <= 0 {
		defaultTimeout = 3 * time.Minute
	}
	prefix := strings.TrimSpace(strings.ToLower(assignmentPrefix))
	if prefix == "" {
		prefix = defaultAssignmentPrefix
	}
	c, err := osconfig.NewOsConfigZonalClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("osconfig zonal client: %w", err)
	}
	return &OSConfigShellRunner{
		client:           c,
		defaultTimeout:   defaultTimeout,
		assignmentPrefix: sanitizeAssignmentPrefix(prefix),
	}, nil
}

// Close closes the underlying client.
func (r *OSConfigShellRunner) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

// RunShellTarget identifies the project, zone, instance, and label selector for OS policy assignment.
type RunShellTarget struct {
	ProjectID        string
	Zone             string
	InstanceName     string
	LabelKey         string
	LabelValue       string
	AssignmentPrefix string
	Timeout          time.Duration
}

func prefixForAssignment(r *OSConfigShellRunner, t RunShellTarget) string {
	if p := strings.TrimSpace(t.AssignmentPrefix); p != "" {
		return sanitizeAssignmentPrefix(p)
	}
	return r.assignmentPrefix
}

// RunShellCommand creates a temporary OS policy assignment with an ExecResource, waits for rollout,
// reads the per-instance report, then deletes the assignment.
func (r *OSConfigShellRunner) RunShellCommand(ctx context.Context, t RunShellTarget, shell string) (*ShellCommandResult, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("os config runner not initialized")
	}
	if strings.TrimSpace(t.ProjectID) == "" || strings.TrimSpace(t.Zone) == "" || strings.TrimSpace(t.InstanceName) == "" {
		return nil, fmt.Errorf("gcp osconfig: project_id, zone, and instance_name are required")
	}
	if strings.TrimSpace(t.LabelKey) == "" || strings.TrimSpace(t.LabelValue) == "" {
		return nil, fmt.Errorf("gcp osconfig: label_key and label_value are required for instance targeting")
	}
	deadline := time.Now().Add(r.timeout(t.Timeout))
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	assignmentID := newOSPolicyAssignmentID(prefixForAssignment(r, t))
	parent := fmt.Sprintf("projects/%s/locations/%s", strings.TrimSpace(t.ProjectID), strings.TrimSpace(t.Zone))

	outPath := fmt.Sprintf("/tmp/caddy-dash-os-%s.log", strings.ReplaceAll(assignmentID, "-", ""))
	enforceScript := buildEnforceScript(shell, outPath)

	assignment := buildOSPolicyAssignment(assignmentID, t.LabelKey, t.LabelValue, enforceScript, outPath)

	createOp, err := r.client.CreateOSPolicyAssignment(ctx, &osconfigpb.CreateOSPolicyAssignmentRequest{
		Parent:               parent,
		OsPolicyAssignment:   assignment,
		OsPolicyAssignmentId: assignmentID,
	})
	if err != nil {
		return nil, fmt.Errorf("create os policy assignment: %w", err)
	}
	created, err := createOp.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("wait create os policy assignment: %w", err)
	}
	fullName := created.GetName()
	defer func() {
		delCtx, delCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer delCancel()
		if delOp, derr := r.client.DeleteOSPolicyAssignment(delCtx, &osconfigpb.DeleteOSPolicyAssignmentRequest{Name: fullName}); derr == nil {
			_ = delOp.Wait(delCtx)
		}
	}()

	if err := r.waitRolloutSucceeded(ctx, fullName); err != nil {
		return &ShellCommandResult{
			Status: "Failed",
			Stderr: err.Error(),
			Meta:   metaForGCP(assignmentID, fullName, ""),
		}, nil
	}

	reportName := fmt.Sprintf("%s/instances/%s/osPolicyAssignments/%s/report",
		parent, strings.TrimSpace(t.InstanceName), assignmentID)

	stdout, stderr, st, err := r.pollReport(ctx, reportName, deadline)
	meta := metaForGCP(assignmentID, fullName, reportName)
	if err != nil {
		if stderr == "" {
			stderr = err.Error()
		}
		return &ShellCommandResult{Status: st, Stdout: stdout, Stderr: stderr, Meta: meta}, nil
	}
	return &ShellCommandResult{Status: st, Stdout: stdout, Stderr: stderr, Meta: meta}, nil
}

func (r *OSConfigShellRunner) timeout(t time.Duration) time.Duration {
	if t > 0 {
		return t
	}
	return r.defaultTimeout
}

func metaForGCP(assignmentID, assignmentName, reportName string) map[string]string {
	return map[string]string{
		"gcp_os_policy_assignment_id": assignmentID,
		"gcp_os_policy_assignment":    assignmentName,
		"gcp_report":                  reportName,
	}
}

func newOSPolicyAssignmentID(prefix string) string {
	suffix := strings.ReplaceAll(uuid.New().String(), "-", "")
	suffix = suffix[:12]
	id := fmt.Sprintf("%s-%s", prefix, suffix)
	if len(id) > 63 {
		id = id[:63]
	}
	// Must end with letter or number; trim trailing hyphen if any.
	id = strings.TrimRight(id, "-")
	return id
}

func sanitizeAssignmentPrefix(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return defaultAssignmentPrefix
	}
	if out[0] < 'a' || out[0] > 'z' {
		out = "c" + out
	}
	if len(out) > 20 {
		out = out[:20]
	}
	out = strings.TrimRight(out, "-")
	if out == "" {
		return defaultAssignmentPrefix
	}
	return out
}

func buildEnforceScript(userShell, outputPath string) string {
	// User command is base64-wrapped to avoid quoting issues in the OS policy JSON/proto path.
	enc := base64.StdEncoding.EncodeToString([]byte(userShell))
	return fmt.Sprintf(`#!/bin/sh
set +e
TMP=/tmp/caddy-dash-inner-$$.b64
cat >"$TMP" <<'B64EOF'
%s
B64EOF
base64 -d <"$TMP" > /tmp/caddy-dash-inner-$$.sh
chmod +x /tmp/caddy-dash-inner-$$.sh
OUT="%s"
rm -f "$OUT"
/bin/sh /tmp/caddy-dash-inner-$$.sh >"$OUT" 2>&1
rc=$?
rm -f "$TMP" /tmp/caddy-dash-inner-$$.sh
if [ "$rc" -eq 0 ]; then exit 100; fi
exit 1
`, enc, outputPath)
}

func buildOSPolicyAssignment(assignmentID, labelKey, labelValue, enforceScript, outputPath string) *osconfigpb.OSPolicyAssignment {
	return &osconfigpb.OSPolicyAssignment{
		Description: "Temporary Caddy Dashboard exec (auto-deleted)",
		InstanceFilter: &osconfigpb.OSPolicyAssignment_InstanceFilter{
			InclusionLabels: []*osconfigpb.OSPolicyAssignment_LabelSet{
				{Labels: map[string]string{strings.TrimSpace(labelKey): strings.TrimSpace(labelValue)}},
			},
		},
		Rollout: &osconfigpb.OSPolicyAssignment_Rollout{
			DisruptionBudget: &osconfigpb.FixedOrPercent{
				Mode: &osconfigpb.FixedOrPercent_Fixed{Fixed: 1},
			},
			MinWaitDuration: durationpb.New(1 * time.Second),
		},
		OsPolicies: []*osconfigpb.OSPolicy{
			{
				Id:   osPolicyIDForCaddy,
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					{
						InventoryFilters: []*osconfigpb.OSPolicy_InventoryFilter{},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: osPolicyResourceID,
								ResourceType: &osconfigpb.OSPolicy_Resource_Exec{
									Exec: &osconfigpb.OSPolicy_Resource_ExecResource{
										Validate: &osconfigpb.OSPolicy_Resource_ExecResource_Exec{
											Source: &osconfigpb.OSPolicy_Resource_ExecResource_Exec_Script{
												Script: "exit 101",
											},
											Interpreter: osconfigpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
										},
										Enforce: &osconfigpb.OSPolicy_Resource_ExecResource_Exec{
											Source: &osconfigpb.OSPolicy_Resource_ExecResource_Exec_Script{
												Script: enforceScript,
											},
											Interpreter:    osconfigpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
											OutputFilePath: outputPath,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *OSConfigShellRunner) waitRolloutSucceeded(ctx context.Context, assignmentName string) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			got, err := r.client.GetOSPolicyAssignment(ctx, &osconfigpb.GetOSPolicyAssignmentRequest{Name: assignmentName})
			if err != nil {
				continue
			}
			switch got.GetRolloutState() {
			case osconfigpb.OSPolicyAssignment_SUCCEEDED:
				return nil
			case osconfigpb.OSPolicyAssignment_CANCELLED:
				return fmt.Errorf("os policy assignment rollout cancelled")
			case osconfigpb.OSPolicyAssignment_CANCELLING:
				continue
			default:
				continue
			}
		}
	}
}

func (r *OSConfigShellRunner) pollReport(ctx context.Context, reportName string, deadline time.Time) (stdout, stderr, status string, err error) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		if time.Now().After(deadline) {
			return "", "", "Failed", fmt.Errorf("timeout waiting for os policy assignment report")
		}
		select {
		case <-ctx.Done():
			return "", "", "Failed", ctx.Err()
		case <-ticker.C:
			rep, gerr := r.client.GetOSPolicyAssignmentReport(ctx, &osconfigpb.GetOSPolicyAssignmentReportRequest{Name: reportName})
			if gerr != nil {
				continue
			}
			for _, pc := range rep.GetOsPolicyCompliances() {
				if pc.GetOsPolicyId() != osPolicyIDForCaddy {
					continue
				}
				switch pc.GetComplianceState() {
				case osconfigpb.OSPolicyAssignmentReport_OSPolicyCompliance_UNKNOWN:
					continue
				case osconfigpb.OSPolicyAssignmentReport_OSPolicyCompliance_NON_COMPLIANT:
					msg := pc.GetComplianceStateReason()
					for _, rc := range pc.GetOsPolicyResourceCompliances() {
						if eo := rc.GetExecResourceOutput(); eo != nil && len(eo.GetEnforcementOutput()) > 0 {
							msg += "\n" + string(eo.GetEnforcementOutput())
						}
					}
					return "", strings.TrimSpace(msg), "Failed", nil
				case osconfigpb.OSPolicyAssignmentReport_OSPolicyCompliance_COMPLIANT:
					var out []byte
					for _, rc := range pc.GetOsPolicyResourceCompliances() {
						if rc.GetOsPolicyResourceId() != osPolicyResourceID {
							continue
						}
						if eo := rc.GetExecResourceOutput(); eo != nil {
							out = eo.GetEnforcementOutput()
						}
					}
					return string(out), "", "Success", nil
				}
			}
		}
	}
}
