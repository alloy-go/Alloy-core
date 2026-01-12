package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type RollbackRequest struct {
	ProjectID          string
	UserID             string
	FailedDeploymentID string // Optional: specific deployment to rollback from
}

type RollbackResult struct {
	RollbackDeploymentID string
	TargetImageTag       string
	TargetCommitSHA      string
	FromDeploymentID     string
	Status               string
}

// convertContainers converts K8s container specs to map format
func convertContainers(containers []v1.Container) []interface{} {
	result := make([]interface{}, len(containers))

	for i, container := range containers {
		containerMap := map[string]interface{}{
			"name":  container.Name,
			"image": container.Image,
		}

		// Add ports if present
		if len(container.Ports) > 0 {
			ports := make([]interface{}, len(container.Ports))
			for j, port := range container.Ports {
				ports[j] = map[string]interface{}{
					"containerPort": port.ContainerPort,
				}
			}
			containerMap["ports"] = ports
		}

		// Add env if present
		if len(container.Env) > 0 {
			envs := make([]interface{}, len(container.Env))
			for j, env := range container.Env {
				envs[j] = map[string]interface{}{
					"name":  env.Name,
					"value": env.Value,
				}
			}
			containerMap["env"] = envs
		}

		// Add envFrom if present (like secretRef)
		if len(container.EnvFrom) > 0 {
			envFroms := make([]interface{}, len(container.EnvFrom))
			for j, envFrom := range container.EnvFrom {
				envFromMap := map[string]interface{}{}
				if envFrom.SecretRef != nil {
					envFromMap["secretRef"] = map[string]interface{}{
						"name": envFrom.SecretRef.Name,
					}
				}
				if envFrom.ConfigMapRef != nil {
					envFromMap["configMapRef"] = map[string]interface{}{
						"name": envFrom.ConfigMapRef.Name,
					}
				}
				envFroms[j] = envFromMap
			}
			containerMap["envFrom"] = envFroms
		}

		// Add readiness probe if present
		if container.ReadinessProbe != nil {
			containerMap["readinessProbe"] = convertProbe(container.ReadinessProbe)
		}

		// Add liveness probe if present
		if container.LivenessProbe != nil {
			containerMap["livenessProbe"] = convertProbe(container.LivenessProbe)
		}

		result[i] = containerMap
	}

	return result
}

// convertProbe converts K8s probe to map format
func convertProbe(probe *v1.Probe) map[string]interface{} {
	probeMap := map[string]interface{}{}

	if probe.HTTPGet != nil {
		probeMap["httpGet"] = map[string]interface{}{
			"path": probe.HTTPGet.Path,
			"port": probe.HTTPGet.Port.IntVal,
		}
	}

	if probe.TCPSocket != nil {
		probeMap["tcpSocket"] = map[string]interface{}{
			"port": probe.TCPSocket.Port.IntVal,
		}
	}

	if probe.InitialDelaySeconds > 0 {
		probeMap["initialDelaySeconds"] = probe.InitialDelaySeconds
	}
	if probe.PeriodSeconds > 0 {
		probeMap["periodSeconds"] = probe.PeriodSeconds
	}
	if probe.FailureThreshold > 0 {
		probeMap["failureThreshold"] = probe.FailureThreshold
	}

	return probeMap
}

// ServiceInfo holds service configuration
type ServiceInfo struct {
	Name  string
	Port  int
	Label string
}

// getServiceInfo fetches service information from cluster
// getServiceInfo fetches the actual service from cluster that matches the deployment
func getServiceInfo(kubeconfigPath, contextName, namespace, deploymentName string) (*ServiceInfo, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Get the deployment to find its pod labels
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		context.Background(),
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	podLabels := deployment.Spec.Template.Labels
	log.Printf("🔍 Deployment labels: %v\n", podLabels)

	// List all services in namespace
	services, err := clientset.CoreV1().Services(namespace).List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	// Find the service whose selector matches the deployment's pod labels
	for _, svc := range services.Items {
		// Skip if no selector (like kubernetes service)
		if len(svc.Spec.Selector) == 0 {
			continue
		}

		// Check if ALL service selectors match deployment pod labels
		allMatch := true
		for selectorKey, selectorValue := range svc.Spec.Selector {
			if podLabels[selectorKey] != selectorValue {
				allMatch = false
				break
			}
		}

		if allMatch {
			log.Printf("✅ Found matching service: %s\n", svc.Name)

			// Get port from service (already in cluster from service.yaml)
			servicePort := 8080 // default
			if len(svc.Spec.Ports) > 0 {
				servicePort = int(svc.Spec.Ports[0].Port)
			}

			// Build label selector from deployment labels
			appLabel := ""
			if appName, exists := podLabels["app"]; exists {
				appLabel = "app=" + appName
			} else {
				// Use first label
				for key, value := range podLabels {
					appLabel = fmt.Sprintf("%s=%s", key, value)
					break
				}
			}

			log.Printf("   Service Port: %d (from service.yaml in cluster)\n", servicePort)
			log.Printf("   Label Selector: %s\n", appLabel)

			return &ServiceInfo{
				Name:  svc.Name,
				Port:  servicePort,
				Label: appLabel,
			}, nil
		}
	}

	return nil, fmt.Errorf("no service found matching deployment %s with labels %v", deploymentName, podLabels)
}

// GetLastSuccessfulDeployment finds the most recent successful deployment
func GetLastSuccessfulDeployment(db *pgxpool.Pool, projectID string, beforeTime time.Time) (*DeploymentRecord, error) {
	query := `
        SELECT deployment_id, project_id, commit_sha, image_tag, 
               namespace, deployment_name, created_at
        FROM deployments
        WHERE project_id = $1 
          AND status = 'ready'
          AND created_at < $2
        ORDER BY created_at DESC
        LIMIT 1
    `

	var record DeploymentRecord
	err := db.QueryRow(context.Background(), query, projectID, beforeTime).Scan(
		&record.DeploymentID,
		&record.ProjectID,
		&record.CommitSHA,
		&record.ImageTag,
		&record.Namespace,
		&record.DeploymentName,
		&record.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("no successful deployment found: %w", err)
	}

	return &record, nil
}

// GetCurrentDeploymentFromCluster fetches the live deployment manifest
func GetCurrentDeploymentFromCluster(kubeconfigPath, contextName, namespace, deploymentName string) (map[string]interface{}, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		context.Background(),
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Convert to unstructured map for easy manipulation
	deploymentMap := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      deployment.Name,
			"namespace": deployment.Namespace,
			"labels":    deployment.Labels,
		},
		"spec": map[string]interface{}{
			"replicas": deployment.Spec.Replicas,
			"selector": map[string]interface{}{
				"matchLabels": deployment.Spec.Selector.MatchLabels,
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": deployment.Spec.Template.Labels,
				},
				"spec": map[string]interface{}{
					"containers": convertContainers(deployment.Spec.Template.Spec.Containers),
				},
			},
		},
	}

	return deploymentMap, nil
}

// ReplaceImageInDeployment updates only the image field
func ReplaceImageInDeployment(deploymentMap map[string]interface{}, newImageTag string) error {
	spec, ok := deploymentMap["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment spec")
	}

	template := spec["template"].(map[string]interface{})
	podSpec := template["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})

	if len(containers) == 0 {
		return fmt.Errorf("no containers found")
	}

	container := containers[0].(map[string]interface{})
	oldImage := container["image"]
	container["image"] = newImageTag

	log.Printf("🔄 Image updated:")
	log.Printf("   From: %v", oldImage)
	log.Printf("   To:   %s", newImageTag)

	return nil
}

// ExecuteRollback performs the complete rollback operation
// ExecuteRollback performs the complete rollback operation
func ExecuteRollback(db *pgxpool.Pool, request RollbackRequest) (*RollbackResult, error) {
	// 1. Get project details
	var configPath, contextName string
	err := db.QueryRow(context.Background(), `
        SELECT u.config_path, p.context_name 
        FROM users u 
        JOIN projects p ON u.user_id = p.user_id 
        WHERE u.user_id = $1 AND p.project_id = $2
    `, request.UserID, request.ProjectID).Scan(&configPath, &contextName)

	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// 2. Get current deployment info
	var currentDeployment DeploymentRecord
	currentQuery := `
        SELECT deployment_id, created_at, namespace, deployment_name
        FROM deployments
        WHERE project_id = $1
        ORDER BY created_at DESC
        LIMIT 1
    `
	err = db.QueryRow(context.Background(), currentQuery, request.ProjectID).Scan(
		&currentDeployment.DeploymentID,
		&currentDeployment.CreatedAt,
		&currentDeployment.Namespace,
		&currentDeployment.DeploymentName,
	)
	if err != nil {
		return nil, fmt.Errorf("no deployments found: %w", err)
	}

	// 3. Find last successful deployment
	rollbackTarget, err := GetLastSuccessfulDeployment(db, request.ProjectID, currentDeployment.CreatedAt)
	if err != nil {
		return nil, err
	}

	log.Printf("🎯 Rollback target found:")
	log.Printf("   Image: %s", rollbackTarget.ImageTag)
	log.Printf("   Commit: %s", rollbackTarget.CommitSHA)
	log.Printf("   Deployed: %s", rollbackTarget.CreatedAt)

	// 4. Get current deployment from cluster
	deploymentMap, err := GetCurrentDeploymentFromCluster(
		configPath,
		contextName,
		currentDeployment.Namespace,
		currentDeployment.DeploymentName,
	)
	if err != nil {
		return nil, err
	}

	// 5. Replace image with rollback target
	err = ReplaceImageInDeployment(deploymentMap, rollbackTarget.ImageTag)
	if err != nil {
		return nil, err
	}

	// 6. Convert back to YAML
	deploymentYAML, err := yaml.Marshal(deploymentMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment: %w", err)
	}

	// 7. Get service info from cluster
	// 7. Get service info from cluster (reads the actual service.yaml that was applied)
	serviceInfo, err := getServiceInfo(
		configPath,
		contextName,
		currentDeployment.Namespace,
		currentDeployment.DeploymentName,
	)
	if err != nil {
		return nil, err
	}

	// 8. Create rollback deployment record
	rollbackDeploymentID := uuid.New().String()
	_, err = db.Exec(context.Background(), `
        INSERT INTO deployments 
        (deployment_id, project_id, commit_sha, image_tag, status, 
         namespace, deployment_name, deployment_type, rollback_from)
        VALUES ($1, $2, $3, $4, 'pending', $5, $6, 'rollback', $7)
    `, rollbackDeploymentID, request.ProjectID, rollbackTarget.CommitSHA,
		rollbackTarget.ImageTag, currentDeployment.Namespace,
		currentDeployment.DeploymentName, currentDeployment.DeploymentID)

	if err != nil {
		return nil, fmt.Errorf("failed to create rollback record: %w", err)
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🔄 APPLYING ROLLBACK TO CLUSTER")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 9. 🔑 THIS IS WHERE kubectl apply HAPPENS!
	// Apply the modified deployment YAML to cluster
	if err := ApplyYAML(configPath, contextName, deploymentYAML); err != nil {
		log.Printf("❌ Rollback deployment failed: %v\n", err)

		// Update status to failed
		updateDeploymentStatus(db, rollbackDeploymentID, "failed", "", "", "failed")
		updateDeploymentError(db, rollbackDeploymentID, err.Error())

		return nil, fmt.Errorf("failed to apply rollback: %w", err)
	}

	log.Println("✅ Rollback deployment applied to cluster")

	// Update status to deploying
	updateDeploymentStatus(db, rollbackDeploymentID, "deploying", "", "", "success")
	
	_, err = db.Exec(context.Background(), `
    UPDATE deployments 
    SET status = 'rolled_back',
        needs_rollback = false,  -- ← RESET FLAG HERE
        updated_at = NOW()
    WHERE deployment_id = $1
	`, request.FailedDeploymentID)

	if err != nil {
		log.Printf("⚠️ Failed to mark original deployment as rolled_back: %v", err)
	}

	// 10. Build config for monitoring
	rollbackConfig := K8sDeployConfig{
		KubeconfigPath: configPath,
		ContextName:    contextName,
		DeploymentYAML: deploymentYAML, // For reference
		DeploymentID:   rollbackDeploymentID,
		Namespace:      currentDeployment.Namespace,
		DeploymentName: currentDeployment.DeploymentName,
		ServiceName:    serviceInfo.Name,
		ServicePort:    serviceInfo.Port,
		AppLabel:       serviceInfo.Label,
		DB:             db,
	}

	// 11. Start monitoring in background (just watches, doesn't deploy)
	go monitorDeploymentStatus(rollbackConfig)

	return &RollbackResult{
		RollbackDeploymentID: rollbackDeploymentID,
		TargetImageTag:       rollbackTarget.ImageTag,
		TargetCommitSHA:      rollbackTarget.CommitSHA,
		FromDeploymentID:     currentDeployment.DeploymentID,
		Status:               "rolling_back",
	}, nil
}

type DeploymentRecord struct {
	DeploymentID   string
	ProjectID      string
	CommitSHA      string
	ImageTag       string
	Namespace      string
	DeploymentName string
	CreatedAt      time.Time
}
