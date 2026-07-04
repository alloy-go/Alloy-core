package k8deploy

import(
	"context"
	"fmt"
	"log"
	
	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


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
