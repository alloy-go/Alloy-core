package types

import (
	"github.com/jackc/pgx/v5/pgxpool"
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