package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/log"
)

// EditResource opens the resource YAML in the user's editor
// and applies changes when saved
func EditResource(ctx context.Context, client *k8s.Client, resourceType, namespace, name string) error {
	// Get the GVR for the resource type
	gvr, ok := client.GetGVR(resourceType)
	if !ok {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Get the YAML for the resource
	yaml, err := client.GetResourceYAML(ctx, namespace, name, gvr)
	if err != nil {
		return fmt.Errorf("failed to get YAML: %w", err)
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("k13s-%s-%s-*.yaml", resourceType, name))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write YAML to temp file
	if _, err := tmpFile.WriteString(yaml); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors
		for _, e := range []string{"vim", "vi", "nano", "code"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found. Set $EDITOR environment variable")
	}

	// Get file info before editing
	beforeInfo, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Open editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Check if file was modified
	afterInfo, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to stat file after edit: %w", err)
	}

	if afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		log.Infof("No changes detected, skipping apply")
		return nil
	}

	// Read modified YAML
	modifiedYAML, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}

	// Apply the changes using kubectl
	return ApplyYAML(ctx, namespace, modifiedYAML)
}

// ApplyYAML applies YAML using kubectl
func ApplyYAML(ctx context.Context, namespace string, yamlData []byte) error {
	// Create a temporary file for the YAML
	tmpFile, err := os.CreateTemp("", "k13s-apply-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(yamlData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Run kubectl apply
	args := []string{"apply", "-f", tmpPath}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}

	log.Infof("Resource updated successfully")
	return nil
}

// EditResourceWithKubectl opens kubectl edit for the resource
func EditResourceWithKubectl(resourceType, namespace, name string) error {
	args := []string{"edit", resourceType, name}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
