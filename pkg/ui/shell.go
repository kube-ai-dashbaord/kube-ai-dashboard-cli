package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kube-ai-dashbaord/kube-ai-dashboard-cli/pkg/k8s"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ShellModal provides a modal for selecting container and shell for exec
type ShellModal struct {
	Form       *tview.Form
	Client     *k8s.Client
	Namespace  string
	PodName    string
	Containers []string
	OnExec     func(namespace, pod, container, shell string)
	OnCancel   func()
}

// NewShellModal creates a new shell selection modal
func NewShellModal(client *k8s.Client, namespace, podName string, containers []string, onExec func(namespace, pod, container, shell string), onCancel func()) *ShellModal {
	m := &ShellModal{
		Form:       tview.NewForm(),
		Client:     client,
		Namespace:  namespace,
		PodName:    podName,
		Containers: containers,
		OnExec:     onExec,
		OnCancel:   onCancel,
	}

	// Default shell options
	shells := []string{"/bin/sh", "/bin/bash", "/bin/zsh", "/bin/ash"}

	// Container dropdown (default to first container)
	containerOptions := containers
	if len(containerOptions) == 0 {
		containerOptions = []string{"(no containers)"}
	}

	selectedContainer := 0
	selectedShell := 0

	m.Form.AddDropDown("Container", containerOptions, 0, func(option string, index int) {
		selectedContainer = index
	})

	m.Form.AddDropDown("Shell", shells, 0, func(option string, index int) {
		selectedShell = index
	})

	m.Form.AddButton("Exec", func() {
		if len(containers) > 0 && m.OnExec != nil {
			m.OnExec(namespace, podName, containers[selectedContainer], shells[selectedShell])
		}
	})

	m.Form.AddButton("Cancel", func() {
		if m.OnCancel != nil {
			m.OnCancel()
		}
	})

	m.Form.SetBorder(true).
		SetTitle(fmt.Sprintf(" Shell into %s/%s ", namespace, podName)).
		SetTitleAlign(tview.AlignCenter)

	m.Form.SetFieldBackgroundColor(tcell.ColorBlack)
	m.Form.SetButtonBackgroundColor(tcell.ColorDarkCyan)

	m.Form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if m.OnCancel != nil {
				m.OnCancel()
			}
			return nil
		}
		return event
	})

	return m
}

// ExecInPod executes a command in a pod container using kubectl exec
// This suspends the TUI and runs in the terminal directly
func ExecInPod(ctx context.Context, client *k8s.Client, namespace, pod, container, shell string) error {
	// Build kubectl exec command
	args := []string{"exec", "-it", "-n", namespace, pod}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--", shell)

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ExecInPodWithClient executes a command using the k8s client directly
// Returns stdout, stderr, and error
func ExecInPodWithClient(ctx context.Context, client *k8s.Client, namespace, pod, container string, command []string) (string, string, error) {
	req := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	restConfig, err := client.GetRestConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to get REST config: %w", err)
	}

	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr strings.Builder
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

// GetPodContainers returns the list of container names in a pod
func GetPodContainers(ctx context.Context, client *k8s.Client, namespace, podName string) ([]string, error) {
	pod, err := client.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, k8s.DefaultGetOptions())
	if err != nil {
		return nil, err
	}

	var containers []string
	for _, c := range pod.Spec.Containers {
		containers = append(containers, c.Name)
	}
	// Also include init containers
	for _, c := range pod.Spec.InitContainers {
		containers = append(containers, c.Name+" (init)")
	}

	return containers, nil
}

// LogsStreamer handles streaming logs from a pod
type LogsStreamer struct {
	Client    *k8s.Client
	Namespace string
	PodName   string
	Container string
	Follow    bool
	TailLines int64
	cancelFn  context.CancelFunc
}

// NewLogsStreamer creates a new logs streamer
func NewLogsStreamer(client *k8s.Client, namespace, podName, container string) *LogsStreamer {
	return &LogsStreamer{
		Client:    client,
		Namespace: namespace,
		PodName:   podName,
		Container: container,
		Follow:    true,
		TailLines: 100,
	}
}

// Stream starts streaming logs to the provided writer
func (l *LogsStreamer) Stream(ctx context.Context, writer io.Writer) error {
	ctx, l.cancelFn = context.WithCancel(ctx)

	opts := &corev1.PodLogOptions{
		Container: l.Container,
		Follow:    l.Follow,
		TailLines: &l.TailLines,
	}

	req := l.Client.Clientset.CoreV1().Pods(l.Namespace).GetLogs(l.PodName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = io.Copy(writer, stream)
	return err
}

// Stop stops the log streaming
func (l *LogsStreamer) Stop() {
	if l.cancelFn != nil {
		l.cancelFn()
	}
}
