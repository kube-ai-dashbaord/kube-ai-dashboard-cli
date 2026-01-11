package k8s

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.yaml.in/yaml/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

type Client struct {
	Clientset kubernetes.Interface
	Dynamic   dynamic.Interface
	Config    *rest.Config
	Metrics   *metricsv1beta1.MetricsV1beta1Client
}

func NewClient() (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	metricsClient, err := metricsv1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset: clientset,
		Dynamic:   dynamicClient,
		Config:    config,
		Metrics:   metricsClient,
	}, nil
}

func (c *Client) SwitchContext(contextName string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	c.Clientset = clientset
	c.Dynamic = dynamicClient
	return nil
}

func (c *Client) ListPods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (c *Client) ListNodes(ctx context.Context) ([]corev1.Node, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
	deps, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return deps.Items, nil
}

func (c *Client) ListStatefulSets(ctx context.Context, namespace string) ([]appsv1.StatefulSet, error) {
	stses, err := c.Clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return stses.Items, nil
}

func (c *Client) ListServices(ctx context.Context, namespace string) ([]corev1.Service, error) {
	svcs, err := c.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return svcs.Items, nil
}

func (c *Client) ListConfigMaps(ctx context.Context, namespace string) ([]corev1.ConfigMap, error) {
	cms, err := c.Clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return cms.Items, nil
}

func (c *Client) ListSecrets(ctx context.Context, namespace string) ([]corev1.Secret, error) {
	secrets, err := c.Clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return secrets.Items, nil
}

func (c *Client) ListIngresses(ctx context.Context, namespace string) ([]networkingv1.Ingress, error) {
	ings, err := c.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ings.Items, nil
}

func (c *Client) ListNamespaces(ctx context.Context) ([]corev1.Namespace, error) {
	ns, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ns.Items, nil
}

func (c *Client) GetPodLogs(ctx context.Context, namespace, name, container string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
	}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}
	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
func (c *Client) GetContextInfo() (ctxName, cluster, user string, err error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return "", "", "", err
	}

	ctxName = rawConfig.CurrentContext
	if ctx, ok := rawConfig.Contexts[ctxName]; ok {
		cluster = ctx.Cluster
		user = ctx.AuthInfo
	}

	return ctxName, cluster, user, nil
}

func (c *Client) GetCurrentContext() (string, error) {
	ctx, _, _, err := c.GetContextInfo()
	return ctx, err
}

func (c *Client) GetCurrentNamespace() string {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	ns, _, _ := kubeConfig.Namespace()
	return ns
}
func (c *Client) GetServerVersion() (string, error) {
	version, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

func (c *Client) ListEvents(ctx context.Context, namespace string) ([]corev1.Event, error) {
	events, err := c.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return events.Items, nil
}

func (c *Client) GetPodLogsStream(ctx context.Context, namespace, name string) (io.ReadCloser, error) {
	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{Follow: true})
	return req.Stream(ctx)
}

func (c *Client) ListTable(ctx context.Context, gvr schema.GroupVersionResource, ns string) (*metav1.Table, error) {
	// For now, return error until REST client implementation is ready
	return nil, fmt.Errorf("dynamic table listing not yet implemented")
}

func (c *Client) ListRoles(ctx context.Context, namespace string) ([]rbacv1.Role, error) {
	roles, err := c.Clientset.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return roles.Items, nil
}

func (c *Client) ListRoleBindings(ctx context.Context, namespace string) ([]rbacv1.RoleBinding, error) {
	rb, err := c.Clientset.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return rb.Items, nil
}

func (c *Client) ListClusterRoles(ctx context.Context) ([]rbacv1.ClusterRole, error) {
	roles, err := c.Clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return roles.Items, nil
}

func (c *Client) ListClusterRoleBindings(ctx context.Context) ([]rbacv1.ClusterRoleBinding, error) {
	crb, err := c.Clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return crb.Items, nil
}

func (c *Client) ListPersistentVolumes(ctx context.Context) ([]corev1.PersistentVolume, error) {
	pv, err := c.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pv.Items, nil
}

func (c *Client) ListPersistentVolumeClaims(ctx context.Context, namespace string) ([]corev1.PersistentVolumeClaim, error) {
	pvc, err := c.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pvc.Items, nil
}

func (c *Client) ListStorageClasses(ctx context.Context) ([]storagev1.StorageClass, error) {
	sc, err := c.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return sc.Items, nil
}

func (c *Client) ListServiceAccounts(ctx context.Context, namespace string) ([]corev1.ServiceAccount, error) {
	sa, err := c.Clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return sa.Items, nil
}

func (c *Client) ListContexts() ([]string, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := loadingRules.Load()
	if err != nil {
		return nil, "", err
	}
	var contexts []string
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, config.CurrentContext, nil
}

func (c *Client) ScaleResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string, replicas int32) error {
	// For deployments, statefulsets, etc.
	payload := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	_, err := c.Dynamic.Resource(gvr).Namespace(namespace).Patch(ctx, name, types.MergePatchType, payload, metav1.PatchOptions{})
	return err
}

func (c *Client) RolloutRestart(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	// Trigger restart by updating annotation
	timestamp := time.Now().Format(time.RFC3339)
	payload := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, timestamp))
	_, err := c.Dynamic.Resource(gvr).Namespace(namespace).Patch(ctx, name, types.MergePatchType, payload, metav1.PatchOptions{})
	return err
}

func (c *Client) PortForward(ctx context.Context, namespace, podName string, localPort, podPort int, stopCh, readyCh chan struct{}) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimLeft(c.Config.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	ports := []string{fmt.Sprintf("%d:%d", localPort, podPort)}
	pf, err := portforward.New(dialer, ports, stopCh, readyCh, nil, nil)
	if err != nil {
		return err
	}

	return pf.ForwardPorts()
}

func (c *Client) DeleteResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	return c.Dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) GetPodMetrics(ctx context.Context, namespace string) (map[string][]int64, error) {
	if c.Metrics == nil {
		return nil, fmt.Errorf("metrics client not initialized")
	}
	podMetrics, err := c.Metrics.PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	res := make(map[string][]int64)
	for _, pm := range podMetrics.Items {
		var cpu, mem int64
		for _, container := range pm.Containers {
			cpu += container.Usage.Cpu().MilliValue()
			mem += container.Usage.Memory().Value() / 1024 / 1024 // MB
		}
		res[pm.Name] = []int64{cpu, mem}
	}
	return res, nil
}

func (c *Client) GetNodeMetrics(ctx context.Context) (map[string][]int64, error) {
	if c.Metrics == nil {
		return nil, fmt.Errorf("metrics client not initialized")
	}
	nodeMetrics, err := c.Metrics.NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	res := make(map[string][]int64)
	for _, nm := range nodeMetrics.Items {
		cpu := nm.Usage.Cpu().MilliValue()
		mem := nm.Usage.Memory().Value() / 1024 / 1024 // MB
		res[nm.Name] = []int64{cpu, mem}
	}
	return res, nil
}

func (c *Client) GetResourceContext(ctx context.Context, ns, name, resource string) (string, error) {
	gvr, ok := c.GetGVR(resource)
	if !ok {
		return "", fmt.Errorf("unknown resource: %s", resource)
	}

	// 1. Get YAML
	yaml, err := c.GetResourceYAML(ctx, ns, name, gvr)
	if err != nil {
		return "", err
	}

	contextBuilder := strings.Builder{}
	contextBuilder.WriteString("### Resource Manifest (YAML)\n")
	contextBuilder.WriteString("```yaml\n")
	contextBuilder.WriteString(yaml)
	contextBuilder.WriteString("\n```\n\n")

	// 2. Get Related Events
	events, _ := c.ListEvents(ctx, ns)
	contextBuilder.WriteString("### Related Events\n")
	foundEvents := false
	for _, ev := range events {
		if ev.InvolvedObject.Name == name || strings.Contains(ev.Message, name) {
			contextBuilder.WriteString(fmt.Sprintf("- [%s] %s: %s\n", ev.LastTimestamp.Format(time.RFC3339), ev.Reason, ev.Message))
			foundEvents = true
		}
	}
	if !foundEvents {
		contextBuilder.WriteString("No related events found.\n")
	}
	contextBuilder.WriteString("\n")

	// 3. If Pod, get Logs (last 20 lines)
	if resource == "pods" || resource == "po" {
		contextBuilder.WriteString("### Recent Logs (Last 20 lines)\n")
		logs, err := c.GetPodLogs(ctx, ns, name, "", 20)
		if err == nil {
			contextBuilder.WriteString("```\n")
			contextBuilder.WriteString(logs)
			contextBuilder.WriteString("\n```\n")
		} else {
			contextBuilder.WriteString(fmt.Sprintf("Error fetching logs: %v\n", err))
		}
	}

	return contextBuilder.String(), nil
}

func (c *Client) GetResourceYAML(ctx context.Context, namespace, name string, gvr schema.GroupVersionResource) (string, error) {
	obj, err := c.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")

	data, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) GetGVR(resource string) (schema.GroupVersionResource, bool) {
	m := map[string]schema.GroupVersionResource{
		"pods":                   {Group: "", Version: "v1", Resource: "pods"},
		"po":                     {Group: "", Version: "v1", Resource: "pods"},
		"services":               {Group: "", Version: "v1", Resource: "services"},
		"svc":                    {Group: "", Version: "v1", Resource: "services"},
		"nodes":                  {Group: "", Version: "v1", Resource: "nodes"},
		"no":                     {Group: "", Version: "v1", Resource: "nodes"},
		"namespaces":             {Group: "", Version: "v1", Resource: "namespaces"},
		"ns":                     {Group: "", Version: "v1", Resource: "namespaces"},
		"deploy":                 {Group: "apps", Version: "v1", Resource: "deployments"},
		"statefulsets":           {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"sts":                    {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"daemonsets":             {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"ds":                     {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"replicasets":            {Group: "apps", Version: "v1", Resource: "replicasets"},
		"rs":                     {Group: "apps", Version: "v1", Resource: "replicasets"},
		"jobs":                   {Group: "batch", Version: "v1", Resource: "jobs"},
		"cronjobs":               {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"cj":                     {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"configmaps":             {Group: "", Version: "v1", Resource: "configmaps"},
		"cm":                     {Group: "", Version: "v1", Resource: "configmaps"},
		"secrets":                {Group: "", Version: "v1", Resource: "secrets"},
		"ingresses":              {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"ing":                    {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"roles":                  {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		"rolebindings":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		"rb":                     {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		"clusterroles":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		"clusterrolebindings":    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		"crb":                    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		"persistentvolumes":      {Group: "", Version: "v1", Resource: "persistentvolumes"},
		"pv":                     {Group: "", Version: "v1", Resource: "persistentvolumes"},
		"persistentvolumeclaims": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"pvc":                    {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"storageclasses":         {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
		"sc":                     {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
		"serviceaccounts":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
		"sa":                     {Group: "", Version: "v1", Resource: "serviceaccounts"},
	}
	gvr, ok := m[strings.ToLower(resource)]
	return gvr, ok
}
