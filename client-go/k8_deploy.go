package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type DeploymentResult struct {
	Secret     string `json:"secret"`
	Service    string `json:"service"`
	Deployment string `json:"deployment"`
}

type K8sDeployConfig struct {
	KubeconfigPath string
	ContextName    string
	SecretYAML     []byte
	ServiceYAML    []byte
	DeploymentYAML []byte
	DeploymentID   string
	Namespace      string
	DeploymentName string
	ServiceName    string
	ServicePort    int
	AppLabel       string
	DB             *pgxpool.Pool
}

// DeployToKubernetes handles the full deployment process
func DeployToKubernetes(config K8sDeployConfig) (*DeploymentResult, error) {
	results := &DeploymentResult{}

	// Update status to deploying
	updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", "", "", "")

	// Deploy Secret
	log.Println("📦 Deploying Secret...")
	if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.SecretYAML); err != nil {
		log.Printf("❌ Secret deployment failed: %v\n", err)
		results.Secret = "failed"
		updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", "failed", "", "")
	} else {
		log.Println("✅ Secret deployed")
		results.Secret = "success"
		updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", "success", "", "")
	}

	// Deploy Service
	log.Println("📦 Deploying Service...")
	if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.ServiceYAML); err != nil {
		log.Printf("❌ Service deployment failed: %v\n", err)
		results.Service = "failed"
		updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", results.Secret, "failed", "")
	} else {
		log.Println("✅ Service deployed")
		results.Service = "success"
		updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", results.Secret, "success", "")
	}

	// Deploy Deployment
	log.Println("📦 Deploying Deployment...")
	if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.DeploymentYAML); err != nil {
		log.Printf("❌ Deployment failed: %v\n", err)
		results.Deployment = "failed"
		updateDeploymentStatus(config.DB, config.DeploymentID, "failed", results.Secret, results.Service, "failed")
		updateDeploymentError(config.DB, config.DeploymentID, err.Error())
		return results, fmt.Errorf("deployment failed: %w", err)
	}

	log.Println("✅ Deployment created")
	results.Deployment = "success"
	updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", results.Secret, results.Service, "success")

	// Start monitoring deployment status in background
	go monitorDeploymentStatus(config)

	return results, nil
}

// waitForSinglePod waits until only one pod remains (old ones terminated)
func waitForSinglePod(kubeconfigPath, contextName, namespace, labelSelector string, timeout time.Duration) error {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		pods, err := clientset.CoreV1().Pods(namespace).List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: labelSelector, // USE DYNAMIC LABEL
			},
		)

		if err != nil {
			return err
		}

		// Count non-terminating pods
		runningPods := 0
		terminatingPods := 0

		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				terminatingPods++
				log.Printf("   ⏳ Pod %s still terminating...\n", pod.Name)
			} else if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		log.Printf("   📊 Running: %d, Terminating: %d\n", runningPods, terminatingPods)

		// Perfect: only new pods, no terminating ones
		if runningPods > 0 && terminatingPods == 0 {
			log.Printf("✅ Old pods cleaned up, %d pod(s) running\n", runningPods)
			return nil
		}

		<-ticker.C
	}

	return fmt.Errorf("timeout waiting for old pods to terminate")
}

// For better Error handling
func execOrFail(ctx context.Context, db *pgxpool.Pool, msg string, query string, args ...any) {
	if _, err := db.Exec(ctx, query, args...); err != nil {
		log.Printf("❌ CRITICAL: %s: %v", msg, err)
		panic(msg) // or log.Fatal(msg)
	}
}

// Monitor deployment status until pods are ready
func monitorDeploymentStatus(config K8sDeployConfig) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	ctx := context.Background()

	timeout := time.After(5 * time.Minute)

	// Grace period: don't check for failures for the first 45 seconds
	// This allows time for image pulls, especially for larger images
	gracePeriod := time.After(45 * time.Second)
	graceExpired := false

	failureCheckTicker := time.NewTicker(5 * time.Second)
	defer failureCheckTicker.Stop()

	for {
		select {
		case <-gracePeriod:
			graceExpired = true
			log.Println("✅ Grace period expired - now checking for failures")

		case <-timeout:
			log.Printf("⏱️  Deployment monitoring timeout for %s\n", config.DeploymentName)

			// Check if this is a rollback deployment
			var deploymentType string
			err := config.DB.QueryRow(ctx, `
                SELECT COALESCE(deployment_type, 'normal') 
                FROM deployments 
                WHERE deployment_id = $1
            `, config.DeploymentID).Scan(&deploymentType)

			if err == nil && deploymentType == "rollback" {
				log.Println("🚨 ROLLBACK DEPLOYMENT TIMED OUT - MANUAL INTERVENTION REQUIRED")

				execOrFail(
					ctx,
					config.DB,
					"rollback timeout persistence failed",
					`
					UPDATE deployments
					SET status = 'failed',
						failure_type = 'hard',
						needs_rollback = false,
						error_message = 'rollback-timeout-5min'
					WHERE deployment_id = $1
					`,
					config.DeploymentID,
				)
			} else {
				log.Println("⏱️  DEPLOYMENT TIMED OUT - TRIGGERING ROLLBACK")

				execOrFail(
					ctx,
					config.DB,
					"timeout rollback flag persistence failed",
					`
					UPDATE deployments
					SET status = 'failed',
						failure_type = 'hard',
						needs_rollback = true,
						error_message = 'timeout-5min'
					WHERE deployment_id = $1
					`,
					config.DeploymentID,
				)
			}
			return

		case <-failureCheckTicker.C:
			// Skip failure checks during grace period
			if !graceExpired {
				continue
			}

			// Check for pod failures (ImagePullBackOff, CrashLoopBackOff, CreateContainerConfigError, etc.)
			if hasFailedPods, reason := checkForFailedPods(config); hasFailedPods {
				log.Printf("🚨 Pod failure detected: %s\n", reason)

				// First, check what type of deployment this is
				var deploymentType string
				err := config.DB.QueryRow(context.Background(), `
                    SELECT COALESCE(deployment_type, 'normal') 
                    FROM deployments 
                    WHERE deployment_id = $1
                `, config.DeploymentID).Scan(&deploymentType)

				if err != nil {
					log.Printf("⚠️  Failed to get deployment type: %v\n", err)
					deploymentType = "normal"
				}

				// If this is already a rollback deployment that failed, don't cascade rollbacks
				if deploymentType == "rollback" {
					log.Println("🚨 ROLLBACK DEPLOYMENT FAILED - STOPPING CASCADE")
					execOrFail(
						ctx,
						config.DB,
						"rollback deployment failure persistence failed",
						`
						UPDATE deployments
						SET status = 'failed',
							failure_type = 'hard',
							needs_rollback = false,
							error_message = $1
						WHERE deployment_id = $2
						`,
						"rollback-failed: "+reason,
						config.DeploymentID,
					)
					return

				}

				// For normal deployments, check if we have a previous successful deployment to rollback to
				var projectID string
				err = config.DB.QueryRow(context.Background(), `
                    SELECT project_id FROM deployments WHERE deployment_id = $1
                `, config.DeploymentID).Scan(&projectID)

				if err != nil {
					log.Printf("❌ Failed to get project ID: %v\n", err)
					return
				}

				// Check if there's any successful deployment to rollback to
				var successfulDeploymentCount int
				err = config.DB.QueryRow(context.Background(), `
                    SELECT COUNT(*) 
                    FROM deployments 
                    WHERE project_id = $1 
                      AND status = 'ready' 
                      AND created_at < (
                          SELECT created_at 
                          FROM deployments 
                          WHERE deployment_id = $2
                      )
                `, projectID, config.DeploymentID).Scan(&successfulDeploymentCount)

				if err != nil {
					log.Printf("❌ Failed to check for previous deployments: %v\n", err)
					execOrFail(ctx, config.DB,
						"First Deployment failure presistent failed",
						`
                        UPDATE deployments 
                        SET status = 'failed', failure_type = 'hard', 
                            needs_rollback = false, error_message = $1
                        WHERE deployment_id = $2
                    `, "First deployment failed "+reason, config.DeploymentID)
					return
				}

				// No previous successful deployments - this is likely the first deployment
				if successfulDeploymentCount == 0 {
					log.Println("⚠️  FIRST DEPLOYMENT FAILED - No previous version to rollback to")
					log.Println("   Please fix the issue and try deploying again")

					execOrFail(ctx, config.DB,
						"First deployment failure",
						`
                        UPDATE deployments 
                        SET status = 'failed', failure_type = 'hard', 
                            needs_rollback = false, 
                            error_message = $1
                        WHERE deployment_id = $2
                    `, "first-deployment-failed: "+reason, config.DeploymentID)
					return
				}

				// We have a previous deployment - proceed with rollback
				log.Printf("✅ Found %d previous successful deployment(s) - proceeding with rollback\n", successfulDeploymentCount)

				// Update database to mark as failed with rollback flag
				execOrFail(
					ctx,
					config.DB,
					"rollback intent persistence failed",
					`
					UPDATE deployments
					SET status = 'failed',
						failure_type = 'hard',
						needs_rollback = true,
						error_message = $1
					WHERE deployment_id = $2
					`,
					reason,
					config.DeploymentID,
				)
				// Get user ID from project
				var userID string
				err = config.DB.QueryRow(context.Background(), `
                    SELECT user_id FROM projects WHERE project_id = $1
                `, projectID).Scan(&userID)

				if err != nil {
					log.Printf("❌ Failed to get user ID: %v\n", err)
					return
				}

				// 🔑 IMMEDIATELY trigger rollback (don't wait for watcher)
				log.Println("🔄 TRIGGERING IMMEDIATE ROLLBACK...")

				rollbackReq := RollbackRequest{
					ProjectID:          projectID,
					UserID:             userID,
					FailedDeploymentID: config.DeploymentID,
				}

				go func() {
					result, rollbackErr := ExecuteRollback(config.DB, rollbackReq)
					if rollbackErr != nil {
						log.Printf("Rollback failed: %v", rollbackErr)

						_, dbErr := config.DB.Exec(context.Background(), `
							UPDATE deployments
							SET needs_rollback = false,
								error_message = $1
							WHERE deployment_id = $2
						`, "auto-rollback-failed: "+rollbackErr.Error(), config.DeploymentID)

						if dbErr != nil {
							log.Printf(
								"Failed to persist auto-rollback failure for deployment %s: %v",
								config.DeploymentID,
								dbErr,
							)
						}

						return
					}
					log.Printf("✅ Rollback completed: %s", result.RollbackDeploymentID)
				}()

				return
			}

		case <-ticker.C:
			status, err := GetDeploymentStatus(
				config.KubeconfigPath,
				config.ContextName,
				config.Namespace,
				config.DeploymentName,
			)

			if err != nil {
				log.Printf("❌ Failed to get deployment status: %v\n", err)
				continue
			}

			log.Printf("📊 Deployment %s status: %s\n", config.DeploymentName, status)

			if status == "ready" {
				log.Printf("✅ Deployment %s is ready!\n", config.DeploymentName)
				updateDeploymentStatus(config.DB, config.DeploymentID, "ready", "", "", "")

				// Clear failure flags on success
				_, dbErr := config.DB.Exec(ctx, `
                    UPDATE deployments SET failure_type = NULL, needs_rollback = false 
                    WHERE deployment_id = $1
                `, config.DeploymentID)

				if dbErr != nil {
					log.Printf("Failed to clear failure flags: %v", err)
				}

				log.Println("⏳ Waiting for old pods to be cleaned up...")
				if err := waitForSinglePod(
					config.KubeconfigPath,
					config.ContextName,
					config.Namespace,
					config.AppLabel,
					30*time.Second,
				); err != nil {
					log.Printf("⚠️  Warning: %v\n", err)
				}

				log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
				log.Println("🔌 STARTING PORT-FORWARD")
				log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

				err := StartPortForward(
					config.KubeconfigPath,
					config.ContextName,
					config.Namespace,
					config.ServiceName,
					config.ServicePort,
					config.ServicePort,
				)

				if err != nil {
					log.Printf("❌ Port-forward failed: %v\n", err)
				} else {
					log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					log.Printf("🎉 DEPLOYMENT COMPLETE - PORT %d IS RUNNING\n", config.ServicePort)
					log.Printf("   Access your app at: http://localhost:%d\n", config.ServicePort)
					log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
				}
				return
			}

			if status == "failed" {
				log.Printf("🚨 %s failed - advanced classification...\n", config.DeploymentName)

				// Use your existing failure_detector.go
				classification, err := ClassifyDeploymentFailure(
					config.KubeconfigPath,
					config.ContextName,
					config.Namespace,
					config.DeploymentName,
				)
				if err != nil {
					classification = &FailureClassification{"soft", "classification-error", true}
				}

				// Store classification
				UpdateDeploymentFailure(config.DB, config.DeploymentID, classification)

				log.Printf("✅ Classified: %s (%s) - Kubernetes Watcher will handle instantly!",
					classification.Type, classification.Reason)
				return
			}
		}
	}
}

// checkForFailedPods checks if any pods are in a failed state and returns the reason
func checkForFailedPods(config K8sDeployConfig) (bool, string) {
	kubeconfigPath := config.KubeconfigPath
	contextName := config.ContextName
	namespace := config.Namespace
	labelSelector := config.AppLabel

	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		log.Printf("⚠️  Failed to load kubeconfig: %v\n", err)
		return false, ""
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Printf("⚠️  Failed to create clientset: %v\n", err)
		return false, ""
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		log.Printf("⚠️  Failed to list pods: %v\n", err)
		return false, ""
	}

	for _, pod := range pods.Items {
		// Check container statuses for errors
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				// These are critical failures that need rollback
				if reason == "ImagePullBackOff" ||
					reason == "ErrImagePull" ||
					reason == "CreateContainerConfigError" ||
					reason == "InvalidImageName" {
					return true, fmt.Sprintf("pod-%s-image-error: %s", pod.Name, reason)
				}
				// Secret/ConfigMap not found
				if strings.Contains(reason, "CreateContainerConfigError") {
					return true, fmt.Sprintf("pod-%s-config-error: %s (%s)", pod.Name, reason, containerStatus.State.Waiting.Message)
				}
			}
			if containerStatus.State.Terminated != nil {
				if containerStatus.State.Terminated.ExitCode != 0 {
					reason := containerStatus.State.Terminated.Reason
					if reason == "Error" || reason == "OOMKilled" {
						return true, fmt.Sprintf("pod-%s-crashed: %s (exit: %d)", pod.Name, reason, containerStatus.State.Terminated.ExitCode)
					}
				}
			}
		}

		// Check pod conditions
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "False" {
				return true, fmt.Sprintf("pod-%s-not-ready: %s (%s)", pod.Name, condition.Reason, condition.Message)
			}
		}
	}

	return false, ""
}

// Update deployment status in database
func updateDeploymentStatus(db *pgxpool.Pool, deploymentID, status, secretStatus, serviceStatus, deploymentStatus string) {
	ctx := context.Background()

	query := `UPDATE deployments SET status = $1, updated_at = NOW()`
	args := []interface{}{status}
	argCount := 2

	if secretStatus != "" {
		query += fmt.Sprintf(`, secret_status = $%d`, argCount)
		args = append(args, secretStatus)
		argCount++
	}
	if serviceStatus != "" {
		query += fmt.Sprintf(`, service_status = $%d`, argCount)
		args = append(args, serviceStatus)
		argCount++
	}
	if deploymentStatus != "" {
		query += fmt.Sprintf(`, deployment_status = $%d`, argCount)
		args = append(args, deploymentStatus)
		argCount++
	}

	query += fmt.Sprintf(` WHERE deployment_id = $%d`, argCount)
	args = append(args, deploymentID)

	_, err := db.Exec(ctx, query, args...) // Add ctx here
	if err != nil {
		log.Printf("❌ Failed to update deployment status: %v\n", err)
	}
}

func updateDeploymentError(db *pgxpool.Pool, deploymentID, errorMsg string) {
	ctx := context.Background()

	// Truncate error message if too long and escape special chars
	if len(errorMsg) > 500 {
		errorMsg = errorMsg[:500] + "..."
	}

	_, err := db.Exec(ctx, `
        UPDATE deployments 
        SET error_message = $1, updated_at = NOW()
        WHERE deployment_id = $2
    `, errorMsg, deploymentID)

	if err != nil {
		log.Printf("❌ Failed to update error message: %v\n", err)
	}
}

// ApplyYAML applies any Kubernetes YAML to the cluster
func ApplyYAML(kubeconfigPath, contextName string, yamlContent []byte) error {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig load error: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("dynamic client error: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("clientset error: %w", err)
	}

	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(yamlContent, nil, obj)
	if err != nil {
		return fmt.Errorf("yaml decode error: %w", err)
	}

	discoveryClient := memory.NewMemCacheClient(clientset.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("rest mapping error: %w", err)
	}

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	ctx := context.Background()
	var dr dynamic.ResourceInterface

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		dr = dynamicClient.Resource(mapping.Resource)
	}

	_, err = dr.Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, err = dr.Update(ctx, obj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("update error: %w", err)
			}
			log.Printf("   ↻ Updated existing %s/%s\n", gvk.Kind, obj.GetName())
			return nil
		}
		return fmt.Errorf("create error: %w", err)
	}

	log.Printf("   ✓ Created %s/%s\n", gvk.Kind, obj.GetName())
	return nil
}

// GetDeploymentStatus checks the status of a deployment
func GetDeploymentStatus(kubeconfigPath, contextName, namespace, deploymentName string) (string, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		context.Background(),
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return "", err
	}

	// Check if rollout is still in progress (controller hasn't seen latest changes)
	if deployment.Generation > deployment.Status.ObservedGeneration {
		return "progressing", nil
	}

	// Check for failed conditions
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == "Progressing" && condition.Reason == "ProgressDeadlineExceeded" {
			return "failed", nil
		}
	}

	// ALL replicas must be updated and available
	// This ensures old pods are gone
	if deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas &&
		deployment.Status.Replicas == *deployment.Spec.Replicas &&
		deployment.Status.AvailableReplicas == *deployment.Spec.Replicas &&
		deployment.Status.ObservedGeneration == deployment.Generation {

		// Double-check: no old ReplicaSets should have pods
		if deployment.Status.UnavailableReplicas == 0 {
			return "ready", nil
		}
	}

	return "progressing", nil
}
